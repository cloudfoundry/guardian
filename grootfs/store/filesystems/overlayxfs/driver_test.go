package overlayxfs_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/docker/docker/pkg/system"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/st3v/glager"
)

var _ = Describe("Driver", func() {
	var (
		storePath     string
		driver        *overlayxfs.Driver
		logger        *lagertest.TestLogger
		spec          image_cloner.ImageDriverSpec
		randomID      string
		randomImageID string
		tardisBinPath string
	)

	BeforeEach(func() {
		tardisBinPath = filepath.Join(os.TempDir(), fmt.Sprintf("tardis-%d", rand.Int()))
		testhelpers.CopyFile(TardisBinPath, tardisBinPath)
		testhelpers.SuidBinary(tardisBinPath)

		randomImageID = testhelpers.NewRandomID()
		randomID = randVolumeID()
		logger = lagertest.NewTestLogger("overlay+xfs")
		var err error
		storePath, err = ioutil.TempDir(StorePath, "")
		Expect(err).ToNot(HaveOccurred())
		driver = overlayxfs.NewDriver(storePath, tardisBinPath, 0)

		Expect(os.MkdirAll(storePath, 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(storePath, store.VolumesDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(storePath, store.MetaDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(storePath, store.ImageDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(storePath, overlayxfs.LinksDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(storePath, overlayxfs.IDDir), 0777)).To(Succeed())

		imagePath := filepath.Join(storePath, store.ImageDirName, randomImageID)
		Expect(os.Mkdir(imagePath, 0755)).To(Succeed())

		spec = image_cloner.ImageDriverSpec{
			ImagePath: imagePath,
			Mount:     true,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(filepath.Join(storePath, store.VolumesDirName))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(storePath, store.ImageDirName))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(storePath, overlayxfs.LinksDirName))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(storePath, overlayxfs.WhiteoutDevice))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(storePath, overlayxfs.IDDir))).To(Succeed())
	})

	Describe("InitFilesystem", func() {
		var fsFile, storePath string

		BeforeEach(func() {
			tempFile, err := ioutil.TempFile("", "xfs-filesystem")
			Expect(err).NotTo(HaveOccurred())
			fsFile = tempFile.Name()
			Expect(os.Truncate(fsFile, 1024*1024*1024)).To(Succeed())

			storePath, err = ioutil.TempDir("", "store")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = syscall.Unmount(storePath, 0)
		})

		It("succcesfully creates and mounts a filesystem", func() {
			Expect(driver.InitFilesystem(logger, fsFile, storePath)).To(Succeed())
			statfs := syscall.Statfs_t{}
			Expect(syscall.Statfs(storePath, &statfs)).To(Succeed())
			Expect(statfs.Type).To(Equal(filesystems.XfsType))
		})

		It("successfully mounts the filesystem with the correct mount options", func() {
			Expect(driver.InitFilesystem(logger, fsFile, storePath)).To(Succeed())
			mountinfo, err := ioutil.ReadFile("/proc/self/mountinfo")
			Expect(err).NotTo(HaveOccurred())

			Expect(string(mountinfo)).To(MatchRegexp(fmt.Sprintf("%s[^\n]*noatime[^\n]*nobarrier[^\n]*prjquota", storePath)))
		})

		Context("when creating the filesystem fails", func() {
			It("returns an error", func() {
				err := driver.InitFilesystem(logger, "/tmp/no-valid", storePath)
				Expect(err).To(MatchError(ContainSubstring("Formatting XFS filesystem")))
			})
		})

		Context("when the filesystem is already formatted", func() {
			BeforeEach(func() {
				cmd := exec.Command("mkfs.xfs", "-f", fsFile)
				Expect(os.Truncate(fsFile, 200*1024*1024)).To(Succeed())
				Expect(cmd.Run()).To(Succeed())
			})

			It("succeeds", func() {
				Expect(driver.InitFilesystem(logger, fsFile, storePath)).To(Succeed())
			})
		})

		Context("when the store is already mounted", func() {
			BeforeEach(func() {
				Expect(os.Truncate(fsFile, 200*1024*1024)).To(Succeed())
				cmd := exec.Command("mkfs.xfs", "-f", fsFile)
				Expect(cmd.Run()).To(Succeed())
				cmd = exec.Command("mount", "-o", "loop,pquota,noatime,nobarrier", "-t", "xfs", fsFile, storePath)
				Expect(cmd.Run()).To(Succeed())
			})

			It("succeeds", func() {
				Expect(driver.InitFilesystem(logger, fsFile, storePath)).To(Succeed())
			})
		})

		Context("when mounting the filesystem fails", func() {
			It("returns an error", func() {
				err := driver.InitFilesystem(logger, fsFile, "/tmp/no-valid")
				Expect(err).To(MatchError(ContainSubstring("Mounting filesystem")))
			})
		})

		Context("when external log size is > 0", func() {
			var logdevPath string

			BeforeEach(func() {
				driver = overlayxfs.NewDriver(storePath, tardisBinPath, 64)
				logdevPath = fmt.Sprintf("%s.external-log", storePath)
			})

			AfterEach(func() {
				_ = syscall.Unmount(storePath, 0)
				_ = exec.Command("sh", "-c", fmt.Sprintf("losetup -j %s | cut -d : -f 1 | xargs losetup -d", logdevPath)).Run()
			})

			It("succcesfully creates a logdev file", func() {
				Expect(logdevPath).ToNot(BeAnExistingFile())
				Expect(driver.InitFilesystem(logger, fsFile, storePath)).To(Succeed())

				stat, err := os.Stat(logdevPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Size()).To(Equal(int64(64 * 1024 * 1024)))
			})

			It("mounts the xfs volume with the logdev and size options", func() {
				Expect(driver.InitFilesystem(logger, fsFile, storePath)).To(Succeed())
				mountinfo, err := ioutil.ReadFile("/proc/self/mountinfo")
				Expect(err).NotTo(HaveOccurred())

				Expect(string(mountinfo)).To(MatchRegexp(fmt.Sprintf("%s[^\n]*logdev", storePath)))
			})

			Context("when it fails to create the external log file", func() {
				BeforeEach(func() {
					Expect(os.MkdirAll(fmt.Sprintf("%s.external-log", storePath), 0755)).To(Succeed())
				})

				It("returns an error", func() {
					err := driver.InitFilesystem(logger, fsFile, storePath)
					Expect(err).To(MatchError(ContainSubstring("truncating external log file")))
				})
			})
		})
	})

	Describe("CreateImage", func() {
		var (
			layer1ID   string
			layer2ID   string
			layer1Path string
			layer2Path string
		)

		BeforeEach(func() {
			layer1ID = randVolumeID()
			layer1Path = createVolume(storePath, driver, "parent-id", layer1ID, 5000)
			Expect(ioutil.WriteFile(filepath.Join(layer1Path, "file-hello"), []byte("hello-1"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(layer1Path, "file-bye"), []byte("bye-1"), 0700)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(layer1Path, "a-folder"), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(layer1Path, "a-folder", "folder-file"), []byte("in-a-folder-1"), 0755)).To(Succeed())

			layer2ID = randVolumeID()
			layer2Path = createVolume(storePath, driver, "parent-id", layer2ID, 10000)
			Expect(ioutil.WriteFile(filepath.Join(layer2Path, "file-bye"), []byte("bye-2"), 0700)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(layer2Path, "a-folder"), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(layer2Path, "a-folder", "folder-file"), []byte("in-a-folder-2"), 0755)).To(Succeed())

			spec.BaseVolumeIDs = []string{layer1ID}
		})

		AfterEach(func() {
			testhelpers.CleanUpOverlayMounts(StorePath)
		})

		It("initializes the image path", func() {
			Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

			_, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).To(BeADirectory())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).To(BeADirectory())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).To(BeADirectory())
		})

		It("creates a rootfs with the same files as the volume", func() {
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

			_, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).To(BeADirectory())

			contents, err := ioutil.ReadFile(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "file-hello"))
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(BeEquivalentTo("hello-1"))

			contents, err = ioutil.ReadFile(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "file-bye"))
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(BeEquivalentTo("bye-1"))

			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "a-folder")).To(BeADirectory())

			contents, err = ioutil.ReadFile(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "a-folder", "folder-file"))
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(BeEquivalentTo("in-a-folder-1"))
		})

		It("returns a mountJson object", func() {
			mountJson, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			Expect(mountJson.Type).To(Equal("overlay"))
			Expect(mountJson.Source).To(Equal("overlay"))
			Expect(mountJson.Destination).To(Equal(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)))
			Expect(mountJson.Options).To(HaveLen(1))
			Expect(mountJson.Options[0]).To(MatchRegexp(fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
				filepath.Join(storePath, overlayxfs.LinksDirName, ".*"),
				filepath.Join(spec.ImagePath, overlayxfs.UpperDir),
				filepath.Join(spec.ImagePath, overlayxfs.WorkDir),
			)))
		})

		Context("when a volume metadata file is missing", func() {
			BeforeEach(func() {
				metaFilePath := filepath.Join(storePath, store.MetaDirName, "volume-"+layer1ID)
				Expect(os.Remove(metaFilePath)).To(Succeed())
			})

			It("logs the occurence but doesn't fail", func() {
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
				Eventually(logger).Should(gbytes.Say("calculating-base-volume-size-failed"))
			})
		})

		Context("multi-layer image", func() {
			BeforeEach(func() {
				spec.BaseVolumeIDs = []string{layer1ID, layer2ID}
			})

			It("creates a rootfs with files correctly composed from the layer volumes", func() {
				Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())

				Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).To(BeADirectory())

				contents, err := ioutil.ReadFile(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "file-hello"))
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEquivalentTo("hello-1"))

				contents, err = ioutil.ReadFile(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "file-bye"))
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEquivalentTo("bye-2"))

				Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "a-folder")).To(BeADirectory())

				contents, err = ioutil.ReadFile(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir, "a-folder", "folder-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEquivalentTo("in-a-folder-2"))
			})
		})

		It("uses the correct permissions to the internal folders", func() {
			_, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			stat, err := os.Stat(filepath.Join(spec.ImagePath, overlayxfs.UpperDir))
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))

			stat, err = os.Stat(filepath.Join(spec.ImagePath, overlayxfs.WorkDir))
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))

			stat, err = os.Stat(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir))
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0755)))
		})

		Context("when Mount is false", func() {
			BeforeEach(func() {
				spec.Mount = false
			})

			It("does not mount the rootfs", func() {
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
				rootfsPath := filepath.Join(spec.ImagePath, "rootfs")
				contents, err := ioutil.ReadDir(rootfsPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(contents).To(BeEmpty())
			})

			It("does still create all the subdirectoies", func() {
				Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).ToNot(BeAnExistingFile())
				Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).ToNot(BeAnExistingFile())
				Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())

				Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).To(BeADirectory())
				Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).To(BeADirectory())
				Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).To(BeADirectory())
			})
		})

		Context("image_info", func() {
			BeforeEach(func() {
				volumeID := randVolumeID()
				createVolume(storePath, driver, "parent-id", volumeID, 5000)

				spec.BaseVolumeIDs = []string{volumeID}
			})

			It("creates a image info file with the total base volume size", func() {
				Expect(filepath.Join(spec.ImagePath, "image_info")).ToNot(BeAnExistingFile())
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(filepath.Join(spec.ImagePath, "image_info")).To(BeAnExistingFile())

				contents, err := ioutil.ReadFile(filepath.Join(spec.ImagePath, "image_info"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(Equal("5000"))
			})
		})

		It("doesn't apply any quota", func() {
			spec.DiskLimit = 0
			_, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("overlay+xfs.overlayxfs-creating-image.applying-quotas.no-need-for-quotas"),
				),
			))
		})

		Context("when disk limit is > 0", func() {
			BeforeEach(func() {
				spec.DiskLimit = 1024 * 1024 * 10
				Expect(ioutil.WriteFile(filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", layer1ID)), []byte(`{"Size": 3145728}`), 0644)).To(Succeed())
			})

			It("creates the backingFsBlockDev device in the `images` parent folder", func() {
				backingFsBlockDevPath := filepath.Join(storePath, "backingFsBlockDev")

				Expect(backingFsBlockDevPath).ToNot(BeAnExistingFile())
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(backingFsBlockDevPath).To(BeAnExistingFile())
			})

			It("can overwrite files from the lowerdirs", func() {
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
				imageRootfsPath := filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)

				dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file", imageRootfsPath), "count=5", "bs=1M")
				sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("allows images to have independent quotas", func() {
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
				imageRootfsPath := filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)

				dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-1", imageRootfsPath), "count=6", "bs=1M")
				sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				anotherSpec := spec
				anotherImagePath, err := ioutil.TempDir(filepath.Join(storePath, store.ImageDirName), "another-image")
				Expect(err).NotTo(HaveOccurred())
				anotherSpec.ImagePath = anotherImagePath
				_, err = driver.CreateImage(logger, anotherSpec)
				Expect(err).ToNot(HaveOccurred())
				anotherImageRootfsPath := filepath.Join(anotherImagePath, overlayxfs.RootfsDir)

				dd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-2", anotherImageRootfsPath), "count=6", "bs=1M")
				sess, err = gexec.Start(dd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
			})

			Context("inclusive quota", func() {
				BeforeEach(func() {
					spec.ExclusiveDiskLimit = false
				})

				It("enforces the quota in the image", func() {
					_, err := driver.CreateImage(logger, spec)
					Expect(err).ToNot(HaveOccurred())
					imageRootfsPath := filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)

					dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-1", imageRootfsPath), "count=5", "bs=1M")
					sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					dd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-2", imageRootfsPath), "count=4", "bs=1M")
					sess, err = gexec.Start(dd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess, 5*time.Second).Should(gexec.Exit(1))
					Eventually(sess.Err).Should(gbytes.Say("No space left on device"))
				})

				Context("when the DiskLimit is smaller than VolumeSize", func() {
					It("returns an error", func() {
						spec.DiskLimit = 4000
						_, err := driver.CreateImage(logger, spec)
						Expect(err).To(MatchError(ContainSubstring("disk limit is smaller than volume size")))
					})
				})

			})

			Context("exclusive quota", func() {
				BeforeEach(func() {
					spec.ExclusiveDiskLimit = true
				})

				Context("when the DiskLimit is smaller than VolumeSize", func() {
					It("succeeds", func() {
						spec.DiskLimit = 1024 * 1024 * 3
						_, err := driver.CreateImage(logger, spec)
						Expect(err).ToNot(HaveOccurred())
					})
				})

				It("enforces the quota in the image", func() {
					_, err := driver.CreateImage(logger, spec)
					Expect(err).ToNot(HaveOccurred())
					imageRootfsPath := filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)

					dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-1", imageRootfsPath), "count=8", "bs=1M")
					sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					dd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-2", imageRootfsPath), "count=8", "bs=1M")
					sess, err = gexec.Start(dd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess, 5*time.Second).Should(gexec.Exit(1))
					Eventually(sess.Err).Should(gbytes.Say("No space left on device"))
				})
			})

			Context("when tardis is not in the path", func() {
				BeforeEach(func() {
					driver = overlayxfs.NewDriver(storePath, "/bin/bananas", 0)
				})

				It("returns an error", func() {
					_, err := driver.CreateImage(logger, spec)
					Expect(err).To(MatchError(ContainSubstring("tardis was not found in the $PATH")))
				})
			})

			Context("when tardis doesn't have the setuid bit set", func() {
				BeforeEach(func() {
					testhelpers.UnsuidBinary(tardisBinPath)
				})

				It("returns an error", func() {
					_, err := driver.CreateImage(logger, spec)
					Expect(err).To(MatchError(ContainSubstring("missing the setuid bit on tardis")))
				})
			})
		})

		Context("when base volume folder does not exist", func() {
			BeforeEach(func() {
				testhelpers.UnsuidBinary(tardisBinPath)
			})

			It("returns an error", func() {
				spec.BaseVolumeIDs = []string{"not-real"}
				_, err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("base volume path does not exist")))
			})
		})

		Context("when image path folder doesn't exist", func() {
			It("returns an error", func() {
				spec.ImagePath = "/not-real"
				_, err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("image path does not exist")))
			})
		})

		Context("when creating the upper folder fails", func() {
			It("returns an error", func() {
				Expect(os.MkdirAll(filepath.Join(spec.ImagePath, overlayxfs.UpperDir), 0755)).To(Succeed())
				_, err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("creating upperdir folder")))
			})
		})

		Context("when creating the workdir folder fails", func() {
			It("returns an error", func() {
				Expect(os.MkdirAll(filepath.Join(spec.ImagePath, overlayxfs.WorkDir), 0755)).To(Succeed())
				_, err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("creating workdir folder")))
			})
		})

		Context("when creating the rootfs folder fails", func() {
			It("returns an error", func() {
				Expect(os.MkdirAll(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir), 0755)).To(Succeed())
				_, err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("creating rootfs folder")))
			})
		})
	})

	Describe("DestroyImage", func() {
		JustBeforeEach(func() {
			volumeID := randVolumeID()
			createVolume(storePath, driver, "parent-id", volumeID, 3145728)

			spec.BaseVolumeIDs = []string{volumeID}
			_, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())
		})

		It("unmounts the rootfs dir", func() {
			Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())
			output, err := exec.Command("mount").Output()
			Expect(err).NotTo(HaveOccurred())

			buffer := bytes.NewBuffer(output)
			scanner := bufio.NewScanner(buffer)
			for scanner.Scan() {
				mountLine := scanner.Text()
				Expect(mountLine).NotTo(ContainSubstring(spec.ImagePath))
			}
		})

		It("removes upper, work and rootfs dir from the image path", func() {
			Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).To(BeADirectory())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).To(BeADirectory())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).To(BeADirectory())

			Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())

			Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())
		})

		Context("projectids", func() {
			BeforeEach(func() {
				spec.DiskLimit = 1000000000
			})

			It("removes the projectid directory for that image", func() {
				projectidsDir := filepath.Join(storePath, overlayxfs.IDDir)
				ids, err := ioutil.ReadDir(projectidsDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(1))

				Expect(driver.DestroyImage(logger, spec.ImagePath)).To(Succeed())

				ids, err = ioutil.ReadDir(projectidsDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(BeEmpty())
			})
		})

		Context("when it fails unmount the rootfs", func() {
			It("returns an error", func() {
				Expect(syscall.Unmount(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir), 0)).To(Succeed())

				err := driver.DestroyImage(logger, spec.ImagePath)
				Expect(err).To(MatchError(ContainSubstring("unmounting rootfs folder")))
			})
		})
	})

	Describe("FetchStats", func() {
		BeforeEach(func() {
			volumeID := randVolumeID()
			createVolume(storePath, driver, "parent-id", volumeID, 3000000)

			spec.BaseVolumeIDs = []string{volumeID}
			spec.DiskLimit = 10 * 1024 * 1024
			_, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/rootfs/file-1", spec.ImagePath), "count=4", "bs=1M")
			sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		AfterEach(func() {
			testhelpers.CleanUpOverlayMounts(storePath)
			Expect(os.RemoveAll(spec.ImagePath)).To(Succeed())
		})

		It("reports the image usage correctly", func() {
			stats, err := driver.FetchStats(logger, spec.ImagePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stats.DiskUsage.ExclusiveBytesUsed).To(Equal(int64(4198400)))
			Expect(stats.DiskUsage.TotalBytesUsed).To(Equal(int64(3000000 + 4198400)))
		})

		Context("when path does not exist", func() {
			var imagePath string

			BeforeEach(func() {
				imagePath = "/tmp/not-here"
			})

			It("returns an error", func() {
				_, err := driver.FetchStats(logger, imagePath)
				Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("image path (%s) doesn't exist", imagePath))))
			})
		})

		Context("when the path doesn't have a quota", func() {
			BeforeEach(func() {
				tmpDir, err := ioutil.TempDir(filepath.Join(storePath, store.ImageDirName), "")
				Expect(err).NotTo(HaveOccurred())
				spec.DiskLimit = 0
				spec.ImagePath = tmpDir
				_, err = driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the exclusive bytes used as 0", func() {
				volumeStats, err := driver.FetchStats(logger, spec.ImagePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(volumeStats.DiskUsage.ExclusiveBytesUsed).To(Equal(int64(0)))
				Expect(volumeStats.DiskUsage.TotalBytesUsed).To(BeNumerically("~", 3000000, 100))
			})
		})

		Context("when the path doesn't have an `image_info` file", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(spec.ImagePath, "image_info"))).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.FetchStats(logger, spec.ImagePath)
				Expect(err).To(MatchError(ContainSubstring("reading image info")))
			})
		})

		Context("when it fails to fetch XFS project ID", func() {
			It("returns an error", func() {
				_, err := driver.FetchStats(logger, "/tmp")
				Expect(err).To(MatchError(ContainSubstring("Failed to get projid for")))
			})
		})
	})

	Describe("VolumePath", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(storePath, store.VolumesDirName, randomID), 0755)).To(Succeed())
		})

		It("returns the volume path when it exists", func() {
			retVolPath, err := driver.VolumePath(logger, randomID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retVolPath).To(Equal(filepath.Join(storePath, store.VolumesDirName, randomID)))
		})

		Context("when the volume does not exist", func() {
			It("returns an error", func() {
				_, err := driver.VolumePath(logger, "non-existent-id")
				Expect(err).To(MatchError(ContainSubstring("volume does not exist")))
			})
		})
	})

	Describe("ConfigureStore", func() {
		const (
			currentUID = 2001
			currentGID = 2002
		)

		var (
			storePath string
		)

		BeforeEach(func() {
			var err error
			storePath, err = ioutil.TempDir(storePath, "configure-store-test")
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates a links directory", func() {
			Expect(driver.ConfigureStore(logger, storePath, currentUID, currentGID)).To(Succeed())
			stat, err := os.Stat(filepath.Join(storePath, overlayxfs.LinksDirName))
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.IsDir()).To(BeTrue())
			Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(currentUID)))
			Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(currentGID)))
		})

		It("creates a whiteout device", func() {
			Expect(driver.ConfigureStore(logger, storePath, currentUID, currentGID)).To(Succeed())

			stat, err := os.Stat(filepath.Join(storePath, overlayxfs.WhiteoutDevice))
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(currentUID)))
			Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(currentGID)))
		})

		Context("when the whiteout 'device' is not a device", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(storePath, 0755)).To(Succeed())
				Expect(ioutil.WriteFile(filepath.Join(storePath, overlayxfs.WhiteoutDevice), []byte{}, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				err := driver.ConfigureStore(logger, storePath, currentUID, currentGID)
				Expect(err).To(MatchError(ContainSubstring("the whiteout device file is not a valid device")))
			})
		})
	})

	Describe("ValidateFileSystem", func() {
		Context("when storepath is a XFS mount", func() {
			It("returns no error", func() {
				Expect(driver.ValidateFileSystem(logger, storePath)).To(Succeed())
			})
		})

		Context("when storepath is not a XFS mount", func() {
			It("returns an error", func() {
				err := driver.ValidateFileSystem(logger, "/mnt/ext4")
				Expect(err).To(MatchError(ContainSubstring("Store path filesystem (/mnt/ext4) is incompatible with requested driver")))
			})
		})
	})

	Describe("CreateVolume", func() {
		It("creates a volume", func() {
			expectedVolumePath := filepath.Join(storePath, store.VolumesDirName, randomID)
			Expect(expectedVolumePath).NotTo(BeAnExistingFile())

			volumePath, err := driver.CreateVolume(logger, "parent-id", randomID)
			Expect(err).NotTo(HaveOccurred())

			Expect(expectedVolumePath).To(BeADirectory())
			Expect(volumePath).To(Equal(expectedVolumePath))

			linkFile := filepath.Join(storePath, overlayxfs.LinksDirName, randomID)
			_, err = os.Stat(linkFile)
			Expect(err).ToNot(HaveOccurred(), "volume link file has not been created")

			linkName, err := ioutil.ReadFile(linkFile)
			Expect(err).ToNot(HaveOccurred(), "failed to read volume link file")

			link := filepath.Join(storePath, overlayxfs.LinksDirName, string(linkName))
			linkStat, err := os.Lstat(link)
			Expect(err).ToNot(HaveOccurred())
			Expect(linkStat.Mode()&os.ModeSymlink).ToNot(
				BeZero(),
				fmt.Sprintf("Volume link %s is not a symlink", link),
			)
			Expect(os.Readlink(link)).To(Equal(volumePath), "Volume link does not point to volume")
		})

		Context("when volume dir doesn't exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(storePath, store.VolumesDirName))).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.CreateVolume(logger, "parent-id", randomID)
				Expect(err).To(MatchError(ContainSubstring("creating volume")))
			})
		})

		Context("when volume already exists", func() {
			BeforeEach(func() {
				Expect(os.Mkdir(filepath.Join(storePath, store.VolumesDirName, randomID), 0755)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.CreateVolume(logger, "parent-id", randomID)
				Expect(err).To(MatchError(ContainSubstring("creating volume")))
			})
		})
	})

	Describe("Volumes", func() {
		var volumesPath string
		BeforeEach(func() {
			volumesPath = filepath.Join(storePath, store.VolumesDirName)
			Expect(os.Mkdir(filepath.Join(volumesPath, "sha256:vol-a"), 0777)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(volumesPath, "sha256:vol-b"), 0777)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(volumesPath)).To(Succeed())
		})

		It("returns a list with existing volumes id", func() {
			volumes, err := driver.Volumes(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(volumes)).To(Equal(2), "incorrect number of volumes")
			Expect(volumes).To(ContainElement("sha256:vol-a"))
			Expect(volumes).To(ContainElement("sha256:vol-b"))
		})

		Context("when fails to list volumes", func() {
			It("returns an error", func() {
				driver := overlayxfs.NewDriver(storePath, tardisBinPath, 0)
				Expect(os.RemoveAll(filepath.Join(storePath, store.VolumesDirName))).To(Succeed())
				_, err := driver.Volumes(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to list volumes")))
			})
		})
	})

	Describe("DestroyVolume", func() {
		var (
			volumeID   string
			volumePath string
		)

		JustBeforeEach(func() {
			volumeID = randVolumeID()
			var err error
			volumePath, err = driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the overlayxfs volume by id", func() {
			Expect(volumePath).To(BeADirectory())

			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
		})

		It("deletes the associated symlink", func() {
			Expect(volumePath).To(BeADirectory())
			linkFilePath := filepath.Join(storePath, overlayxfs.LinksDirName, volumeID)
			Expect(linkFilePath).To(BeAnExistingFile())
			linkfileinfo, err := ioutil.ReadFile(linkFilePath)
			Expect(err).ToNot(HaveOccurred())
			symlinkPath := filepath.Join(storePath, overlayxfs.LinksDirName, string(linkfileinfo))
			Expect(symlinkPath).To(BeAnExistingFile())

			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
			Expect(linkFilePath).ToNot(BeAnExistingFile())
			Expect(symlinkPath).ToNot(BeAnExistingFile())
		})

		It("deletes the metadata file", func() {
			metaFilePath := filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", volumeID))
			Expect(ioutil.WriteFile(metaFilePath, []byte{}, 0644)).To(Succeed())
			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(metaFilePath).ToNot(BeAnExistingFile())
		})

		Context("during garbage collection", func() {
			var metaFilePath string

			JustBeforeEach(func() {
				metaFilePath = filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", volumeID))
				Expect(driver.MoveVolume(logger,
					filepath.Join(storePath, store.VolumesDirName, volumeID),
					filepath.Join(storePath, store.VolumesDirName, "gc."+volumeID),
				)).To(Succeed())
				volumeID = "gc." + volumeID
			})

			It("deletes the metadata file", func() {
				Expect(ioutil.WriteFile(metaFilePath, []byte{}, 0644)).To(Succeed())
				Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
				Expect(metaFilePath).ToNot(BeAnExistingFile())
			})
		})

		Context("when removing the metadata file fails", func() {
			It("doesn't return an error, but logs the incident", func() {
				metaFilePath := filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", volumeID))
				Expect(metaFilePath).ToNot(BeAnExistingFile())
				Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
				Expect(logger.Buffer()).To(gbytes.Say("deleting-metadata-file-failed"))
			})
		})

		Context("when the associated symlink has already been deleted", func() {
			It("does not fail", func() {
				linkFilePath := filepath.Join(storePath, overlayxfs.LinksDirName, volumeID)
				Expect(linkFilePath).To(BeAnExistingFile())
				linkfileinfo, err := ioutil.ReadFile(linkFilePath)
				Expect(err).ToNot(HaveOccurred())
				symlinkPath := filepath.Join(storePath, overlayxfs.LinksDirName, string(linkfileinfo))
				Expect(symlinkPath).To(BeAnExistingFile())
				Expect(os.Remove(symlinkPath)).To(Succeed())

				Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
				Expect(volumePath).ToNot(BeAnExistingFile())
			})
		})

		Context("when the associated link file has already been deleted", func() {
			It("does not fail", func() {
				linkFilePath := filepath.Join(storePath, overlayxfs.LinksDirName, volumeID)
				Expect(linkFilePath).To(BeAnExistingFile())
				Expect(os.Remove(linkFilePath)).To(Succeed())

				Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
				Expect(volumePath).ToNot(BeAnExistingFile())
			})
		})
	})

	Describe("MoveVolume", func() {
		var (
			volumeID   string
			volumePath string
		)

		JustBeforeEach(func() {
			volumeID = randVolumeID()
			var err error
			volumePath, err = driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())
		})

		It("moves the volume to the given location", func() {
			newVolumePath := fmt.Sprintf("%s-new", volumePath)

			_, err := os.Stat(newVolumePath)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())

			err = driver.MoveVolume(logger, volumePath, newVolumePath)
			Expect(err).ToNot(HaveOccurred())
			stat, err := os.Stat(newVolumePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(stat.IsDir()).To(BeTrue())
		})

		It("updates the volume link to point to the new volume location", func() {
			newVolumePath := fmt.Sprintf("%s-new", volumePath)
			_, err := os.Stat(newVolumePath)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())
			fileInVolume := "file-in-volume"
			filePath := filepath.Join(volumePath, fileInVolume)
			f, err := os.Create(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.Close()).To(Succeed())

			err = driver.MoveVolume(logger, volumePath, newVolumePath)
			Expect(err).ToNot(HaveOccurred())

			linkName, err := ioutil.ReadFile(filepath.Join(storePath, overlayxfs.LinksDirName, filepath.Base(newVolumePath)))
			Expect(err).NotTo(HaveOccurred())
			linkPath := filepath.Join(storePath, overlayxfs.LinksDirName, string(linkName))
			_, err = os.Lstat(linkPath)
			Expect(err).NotTo(HaveOccurred())

			target, err := os.Readlink(linkPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(target).To(Equal(newVolumePath))

			_, err = os.Stat(filepath.Join(storePath, overlayxfs.LinksDirName, filepath.Base(newVolumePath)))
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the source volume does not exist", func() {
			It("returns an error", func() {
				newVolumePath := fmt.Sprintf("%s-new", volumePath)
				err := driver.MoveVolume(logger, "nonsense", newVolumePath)
				Expect(err).To(MatchError(ContainSubstring("moving volume")))
			})
		})

		Context("when the target volume already exists", func() {
			It("returns without error", func() {
				err := driver.MoveVolume(logger, volumePath, filepath.Dir(volumePath))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("HandleOpaqueWhiteouts", func() {
		var (
			opaqueWhiteouts []string
			volumeID        string
			volumePath      string
		)

		JustBeforeEach(func() {
			volumeID = randVolumeID()
			var err error
			volumePath, err = driver.CreateVolume(logger, "", volumeID)
			Expect(err).NotTo(HaveOccurred())

			Expect(os.MkdirAll(filepath.Join(volumePath, "a/b"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "a/b/file_1"), []byte{}, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "a/b/file_2"), []byte{}, 0755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(volumePath, "c/d/e"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "c/d/file_1"), []byte{}, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "c/d/file_2"), []byte{}, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "c/d/e/file_3"), []byte{}, 0755)).To(Succeed())

			opaqueWhiteouts = []string{
				"/a/b/.opaque",
				"c/d/.opaque",
			}
		})

		It("empties the given folders within a volume", func() {
			Expect(driver.HandleOpaqueWhiteouts(logger, volumeID, opaqueWhiteouts)).To(Succeed())

			abFolderPath := filepath.Join(volumePath, "a/b")
			Expect(abFolderPath).To(BeADirectory())
			files, err := ioutil.ReadDir(abFolderPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())
			xattr, err := system.Lgetxattr(abFolderPath, "trusted.overlay.opaque")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(xattr)).To(Equal("y"))

			cdFolderPath := filepath.Join(volumePath, "c/d")
			Expect(cdFolderPath).To(BeADirectory())
			files, err = ioutil.ReadDir(cdFolderPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(BeEmpty())
			xattr, err = system.Lgetxattr(cdFolderPath, "trusted.overlay.opaque")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(xattr)).To(Equal("y"))
		})
	})

	Describe("WriteVolumeMeta", func() {
		It("creates the correct metadata file", func() {
			err := driver.WriteVolumeMeta(logger, "1234", base_image_puller.VolumeMeta{Size: 1024})
			Expect(err).NotTo(HaveOccurred())

			metaFilePath := filepath.Join(storePath, store.MetaDirName, "volume-1234")
			Expect(metaFilePath).To(BeAnExistingFile())
			metaFile, err := os.Open(metaFilePath)
			Expect(err).NotTo(HaveOccurred())
			var meta base_image_puller.VolumeMeta

			Expect(json.NewDecoder(metaFile).Decode(&meta)).To(Succeed())
			Expect(meta).To(Equal(base_image_puller.VolumeMeta{Size: 1024}))
		})
	})
})

func randVolumeID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

func createVolume(storePath string, driver *overlayxfs.Driver, parentID, id string, size int64) string {
	path, err := driver.CreateVolume(lagertest.NewTestLogger("test"), parentID, id)
	Expect(err).NotTo(HaveOccurred())
	metaFilePath := filepath.Join(storePath, store.MetaDirName, fmt.Sprintf("volume-%s", id))
	metaContents := fmt.Sprintf(`{"Size": %d}`, size)
	Expect(ioutil.WriteFile(metaFilePath, []byte(metaContents), 0644)).To(Succeed())

	return path
}

package overlayxfs_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	. "github.com/st3v/glager"
)

var _ = Describe("Driver", func() {
	var (
		driver        *overlayxfs.Driver
		logger        *lagertest.TestLogger
		spec          image_cloner.ImageDriverSpec
		randomID      string
		tardisBinPath string
	)

	BeforeEach(func() {
		tardisBinPath = filepath.Join(os.TempDir(), fmt.Sprintf("tardis-%d", rand.Int()))
		testhelpers.CopyFile(TardisBinPath, tardisBinPath)
		testhelpers.SuidBinary(tardisBinPath)

		randomID = randVolumeID()
		logger = lagertest.NewTestLogger("overlay+xfs")
		driver = overlayxfs.NewDriver(StorePath, tardisBinPath)

		Expect(os.MkdirAll(StorePath, 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, store.VolumesDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, store.ImageDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, overlayxfs.LinksDirName), 0777)).To(Succeed())

		imagePath := filepath.Join(StorePath, store.ImageDirName, fmt.Sprintf("random-id-%d", rand.Int()))
		Expect(os.Mkdir(imagePath, 0755)).To(Succeed())

		spec = image_cloner.ImageDriverSpec{
			ImagePath: imagePath,
			Mount:     true,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(filepath.Join(StorePath, store.VolumesDirName))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(StorePath, store.ImageDirName))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(StorePath, overlayxfs.LinksDirName))).To(Succeed())
		Expect(os.RemoveAll(filepath.Join(StorePath, overlayxfs.WhiteoutDevice))).To(Succeed())
	})

	Describe("CreateImage", func() {
		var (
			layer1ID   string
			layer2ID   string
			layer1Path string
			layer2Path string
		)

		BeforeEach(func() {
			var err error
			layer1ID = randVolumeID()
			layer1Path, err = driver.CreateVolume(logger, "parent-id", layer1ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(layer1Path, "file-hello"), []byte("hello-1"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(layer1Path, "file-bye"), []byte("bye-1"), 0700)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(layer1Path, "a-folder"), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(layer1Path, "a-folder", "folder-file"), []byte("in-a-folder-1"), 0755)).To(Succeed())

			layer2ID = randVolumeID()
			layer2Path, err = driver.CreateVolume(logger, "parent-id", layer2ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(layer2Path, "file-bye"), []byte("bye-2"), 0700)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(layer2Path, "a-folder"), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(layer2Path, "a-folder", "folder-file"), []byte("in-a-folder-2"), 0755)).To(Succeed())

			spec.BaseVolumeIDs = []string{layer1ID}
		})

		AfterEach(func() {
			testhelpers.CleanUpOverlayMounts(StorePath, store.ImageDirName)
			Expect(os.RemoveAll(spec.ImagePath)).To(Succeed())
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
				filepath.Join(StorePath, overlayxfs.LinksDirName, ".*"),
				filepath.Join(spec.ImagePath, overlayxfs.UpperDir),
				filepath.Join(spec.ImagePath, overlayxfs.WorkDir),
			)))
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
				var err error
				layer1Path, err = driver.CreateVolume(logger, "parent-id", volumeID)
				Expect(err).NotTo(HaveOccurred())

				dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file", layer1Path), "count=3", "bs=1024")
				sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				spec.BaseVolumeIDs = []string{volumeID}
			})

			It("creates a image info file with the total base volume size", func() {
				Expect(filepath.Join(spec.ImagePath, "image_info")).ToNot(BeAnExistingFile())
				_, err := driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(filepath.Join(spec.ImagePath, "image_info")).To(BeAnExistingFile())

				contents, err := ioutil.ReadFile(filepath.Join(spec.ImagePath, "image_info"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(Equal("3090"))
			})
		})

		It("doesn't apply any quota", func() {
			spec.DiskLimit = 0
			_, err := driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			Expect(logger).To(ContainSequence(
				Info(
					Message("overlay+xfs.overlayxfs-creating-image.applying-quotas.no-need-for-quotas"),
				),
			))
		})

		Context("when disk limit is > 0", func() {
			BeforeEach(func() {
				dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file", layer1Path), "count=3", "bs=1M")
				sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				spec.DiskLimit = 1024 * 1024 * 10
			})

			It("creates the backingFsBlockDev device in the `images` parent folder", func() {
				backingFsBlockDevPath := filepath.Join(StorePath, "backingFsBlockDev")

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
				anotherImagePath, err := ioutil.TempDir(filepath.Join(StorePath, store.ImageDirName), "another-image")
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
					spec.DiskLimit = 1024 * 1024 * 10
				})

				Context("when the DiskLimit is smaller than VolumeSize", func() {
					It("returns an error", func() {
						spec.DiskLimit = 1024 * 1024 * 3
						_, err := driver.CreateImage(logger, spec)
						Expect(err).To(MatchError(ContainSubstring("disk limit is smaller than volume size")))
					})
				})

				It("enforces the quota in the image", func() {
					_, err := driver.CreateImage(logger, spec)
					Expect(err).ToNot(HaveOccurred())
					imageRootfsPath := filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)

					dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-1", imageRootfsPath), "count=5", "bs=1M")
					sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(0))

					dd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-2", imageRootfsPath), "count=5", "bs=1M")
					sess, err = gexec.Start(dd, GinkgoWriter, GinkgoWriter)
					Expect(err).NotTo(HaveOccurred())
					Eventually(sess, 5*time.Second).Should(gexec.Exit(1))
					Eventually(sess.Err).Should(gbytes.Say("No space left on device"))
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
					driver = overlayxfs.NewDriver(StorePath, "/bin/bananas")
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
		BeforeEach(func() {
			volumeID := randVolumeID()
			_, err := driver.CreateVolume(logger, "parent-id", volumeID)
			Expect(err).NotTo(HaveOccurred())
			spec.BaseVolumeIDs = []string{volumeID}
			_, err = driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())
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
			layer1Path, err := driver.CreateVolume(logger, "parent-id", volumeID)
			Expect(err).NotTo(HaveOccurred())

			dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file", layer1Path), "count=3", "bs=1M")
			sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			spec.BaseVolumeIDs = []string{volumeID}
			spec.DiskLimit = 10 * 1024 * 1024
			_, err = driver.CreateImage(logger, spec)
			Expect(err).ToNot(HaveOccurred())

			dd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/rootfs/file-1", spec.ImagePath), "count=4", "bs=1M")
			sess, err = gexec.Start(dd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		AfterEach(func() {
			testhelpers.CleanUpOverlayMounts(StorePath, store.ImageDirName)
			Expect(os.RemoveAll(spec.ImagePath)).To(Succeed())
		})

		It("reports the image usage correctly", func() {
			stats, err := driver.FetchStats(logger, spec.ImagePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(stats.DiskUsage.ExclusiveBytesUsed).To(Equal(int64(4198400)))
			Expect(stats.DiskUsage.TotalBytesUsed).To(Equal(int64(7344146)))
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
				tmpDir, err := ioutil.TempDir(filepath.Join(StorePath, store.ImageDirName), "")
				Expect(err).NotTo(HaveOccurred())
				spec.DiskLimit = 0
				spec.ImagePath = tmpDir
				_, err = driver.CreateImage(logger, spec)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error", func() {
				volumeStats, err := driver.FetchStats(logger, spec.ImagePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(volumeStats.DiskUsage.ExclusiveBytesUsed).To(Equal(int64(0)))
				Expect(volumeStats.DiskUsage.TotalBytesUsed).To(BeNumerically("~", 3*1024*1024, 100))
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
			Expect(os.MkdirAll(filepath.Join(StorePath, store.VolumesDirName, randomID), 0755)).To(Succeed())
		})

		It("returns the volume path when it exists", func() {
			retVolPath, err := driver.VolumePath(logger, randomID)
			Expect(err).NotTo(HaveOccurred())
			Expect(retVolPath).To(Equal(filepath.Join(StorePath, store.VolumesDirName, randomID)))
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
			storePath, err = ioutil.TempDir(StorePath, "configure-store-test")
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
				Expect(driver.ValidateFileSystem(logger, StorePath)).To(Succeed())
			})
		})

		Context("when storepath is not a XFS mount", func() {
			It("returns an error", func() {
				err := driver.ValidateFileSystem(logger, "/mnt/ext4")
				Expect(err).To(MatchError(ContainSubstring("Store path filesystem (/mnt/ext4) is incompatible with requested driver")))
			})
		})
	})

	Describe("Create", func() {
		It("creates a volume", func() {
			expectedVolumePath := filepath.Join(StorePath, store.VolumesDirName, randomID)
			Expect(expectedVolumePath).NotTo(BeAnExistingFile())

			volumePath, err := driver.CreateVolume(logger, "parent-id", randomID)
			Expect(err).NotTo(HaveOccurred())

			Expect(expectedVolumePath).To(BeADirectory())
			Expect(volumePath).To(Equal(expectedVolumePath))

			linkFile := filepath.Join(StorePath, overlayxfs.LinksDirName, randomID)
			_, err = os.Stat(linkFile)
			Expect(err).ToNot(HaveOccurred(), "volume link file has not been created")

			linkName, err := ioutil.ReadFile(linkFile)
			Expect(err).ToNot(HaveOccurred(), "failed to read volume link file")

			link := filepath.Join(StorePath, overlayxfs.LinksDirName, string(linkName))
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
				Expect(os.RemoveAll(filepath.Join(StorePath, store.VolumesDirName))).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.CreateVolume(logger, "parent-id", randomID)
				Expect(err).To(MatchError(ContainSubstring("creating volume")))
			})
		})

		Context("when volume already exists", func() {
			BeforeEach(func() {
				Expect(os.Mkdir(filepath.Join(StorePath, store.VolumesDirName, randomID), 0755)).To(Succeed())
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
			volumesPath = filepath.Join(StorePath, store.VolumesDirName)
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
				driver := overlayxfs.NewDriver(StorePath, tardisBinPath)
				Expect(os.RemoveAll(filepath.Join(StorePath, store.VolumesDirName))).To(Succeed())
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
			linkFilePath := filepath.Join(StorePath, overlayxfs.LinksDirName, volumeID)
			Expect(linkFilePath).To(BeAnExistingFile())
			linkfileinfo, err := ioutil.ReadFile(linkFilePath)
			symlinkPath := filepath.Join(StorePath, overlayxfs.LinksDirName, string(linkfileinfo))
			Expect(err).ToNot(HaveOccurred())
			Expect(symlinkPath).To(BeAnExistingFile())

			Expect(driver.DestroyVolume(logger, volumeID)).To(Succeed())
			Expect(volumePath).ToNot(BeAnExistingFile())
			Expect(linkFilePath).ToNot(BeAnExistingFile())
			Expect(symlinkPath).ToNot(BeAnExistingFile())
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

			stat, err := os.Stat(newVolumePath)
			Expect(err).To(HaveOccurred())
			Expect(os.IsNotExist(err)).To(BeTrue())

			err = driver.MoveVolume(logger, volumePath, newVolumePath)
			Expect(err).ToNot(HaveOccurred())
			stat, err = os.Stat(newVolumePath)
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
			f.Close()

			err = driver.MoveVolume(logger, volumePath, newVolumePath)
			Expect(err).ToNot(HaveOccurred())

			linkName, err := ioutil.ReadFile(filepath.Join(StorePath, overlayxfs.LinksDirName, filepath.Base(newVolumePath)))
			Expect(err).NotTo(HaveOccurred())
			linkPath := filepath.Join(StorePath, overlayxfs.LinksDirName, string(linkName))
			_, err = os.Lstat(linkPath)
			Expect(err).NotTo(HaveOccurred())

			target, err := os.Readlink(linkPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(target).To(Equal(newVolumePath))

			_, err = os.Stat(filepath.Join(StorePath, overlayxfs.LinksDirName, filepath.Base(newVolumePath)))
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
			It("returns an error", func() {
				err := driver.MoveVolume(logger, volumePath, filepath.Dir(volumePath))
				Expect(err).To(MatchError(ContainSubstring("moving volume")))
			})
		})
	})
})

func randVolumeID() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("volume-%d", r.Int())
}

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

var _ = Describe("ImageDriver", func() {
	var (
		driver     *overlayxfs.Driver
		logger     *lagertest.TestLogger
		layer1ID   string
		layer2ID   string
		layer1Path string
		layer2Path string
		spec       image_cloner.ImageDriverSpec
	)

	BeforeEach(func() {
		Expect(os.MkdirAll(StorePath, 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, store.VolumesDirName), 0777)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(StorePath, store.ImageDirName), 0777)).To(Succeed())

		driver = overlayxfs.NewDriver("", StorePath)
		logger = lagertest.NewTestLogger("overlay+xfs")

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

		imagePath := filepath.Join(StorePath, store.ImageDirName, fmt.Sprintf("random-id-%d", rand.Int()))
		Expect(os.Mkdir(imagePath, 0755)).To(Succeed())

		spec = image_cloner.ImageDriverSpec{
			ImagePath:     imagePath,
			BaseVolumeIDs: []string{layer1ID},
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(layer1Path)).To(Succeed())
	})

	Describe("CreateImage", func() {
		AfterEach(func() {
			testhelpers.CleanUpOverlayMounts(StorePath, store.ImageDirName)
			Expect(os.RemoveAll(spec.ImagePath)).To(Succeed())
		})

		It("initializes the image path", func() {
			Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

			Expect(driver.CreateImage(logger, spec)).To(Succeed())

			Expect(filepath.Join(spec.ImagePath, overlayxfs.UpperDir)).To(BeADirectory())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.WorkDir)).To(BeADirectory())
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).To(BeADirectory())
		})

		It("creates a rootfs with the same files as the volume", func() {
			Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

			Expect(driver.CreateImage(logger, spec)).To(Succeed())

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

		Context("multi-layer image", func() {
			BeforeEach(func() {
				spec.BaseVolumeIDs = []string{layer1ID, layer2ID}
			})

			It("creates a rootfs with files correctly composed from the layer volumes", func() {
				Expect(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

				Expect(driver.CreateImage(logger, spec)).To(Succeed())

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
			Expect(driver.CreateImage(logger, spec)).To(Succeed())

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
				Expect(driver.CreateImage(logger, spec)).To(Succeed())
				Expect(filepath.Join(spec.ImagePath, "image_info")).To(BeAnExistingFile())

				contents, err := ioutil.ReadFile(filepath.Join(spec.ImagePath, "image_info"))
				Expect(err).NotTo(HaveOccurred())

				Expect(string(contents)).To(Equal("3090"))
			})
		})

		It("doesn't apply any quota", func() {
			spec.DiskLimit = 0
			Expect(driver.CreateImage(logger, spec)).To(Succeed())

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

			It("can overwrite files from the lowerdirs", func() {
				Expect(driver.CreateImage(logger, spec)).To(Succeed())
				imageRootfsPath := filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)

				dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file", imageRootfsPath), "count=5", "bs=1M")
				sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
			})

			It("allows images to have independent quotas", func() {
				Expect(driver.CreateImage(logger, spec)).To(Succeed())
				imageRootfsPath := filepath.Join(spec.ImagePath, overlayxfs.RootfsDir)

				dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file-1", imageRootfsPath), "count=6", "bs=1M")
				sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				anotherSpec := spec
				anotherImagePath, err := ioutil.TempDir(filepath.Join(StorePath, store.ImageDirName), "another-image")
				Expect(err).NotTo(HaveOccurred())
				anotherSpec.ImagePath = anotherImagePath
				Expect(driver.CreateImage(logger, anotherSpec)).To(Succeed())
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
						Expect(driver.CreateImage(logger, spec)).To(MatchError(ContainSubstring("disk limit is smaller than volume size")))
					})
				})

				It("enforces the quota in the image", func() {
					Expect(driver.CreateImage(logger, spec)).To(Succeed())
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
						Expect(driver.CreateImage(logger, spec)).To(Succeed())
					})
				})

				It("enforces the quota in the image", func() {
					Expect(driver.CreateImage(logger, spec)).To(Succeed())
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
		})

		Context("when base volume folder does not exist", func() {
			It("returns an error", func() {
				spec.BaseVolumeIDs = []string{"not-real"}
				err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("base volume path does not exist")))
			})
		})

		Context("when image path folder doesn't exist", func() {
			It("returns an error", func() {
				spec.ImagePath = "/not-real"
				err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("image path does not exist")))
			})
		})

		Context("when creating the upper folder fails", func() {
			It("returns an error", func() {
				Expect(os.MkdirAll(filepath.Join(spec.ImagePath, overlayxfs.UpperDir), 0755)).To(Succeed())
				err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("creating upperdir folder")))
			})
		})

		Context("when creating the workdir folder fails", func() {
			It("returns an error", func() {
				Expect(os.MkdirAll(filepath.Join(spec.ImagePath, overlayxfs.WorkDir), 0755)).To(Succeed())
				err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("creating workdir folder")))
			})
		})

		Context("when creating the rootfs folder fails", func() {
			It("returns an error", func() {
				Expect(os.MkdirAll(filepath.Join(spec.ImagePath, overlayxfs.RootfsDir), 0755)).To(Succeed())
				err := driver.CreateImage(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("creating rootfs folder")))
			})
		})
	})

	Describe("DestroyImage", func() {
		BeforeEach(func() {
			Expect(driver.CreateImage(logger, spec)).To(Succeed())
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
			var err error
			layer1Path, err = driver.CreateVolume(logger, "parent-id", volumeID)
			Expect(err).NotTo(HaveOccurred())

			dd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s/file", layer1Path), "count=3", "bs=1M")
			sess, err := gexec.Start(dd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			spec.BaseVolumeIDs = []string{volumeID}
			spec.DiskLimit = 10 * 1024 * 1024
			Expect(driver.CreateImage(logger, spec)).To(Succeed())

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
				Expect(driver.CreateImage(logger, spec)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := driver.FetchStats(logger, spec.ImagePath)
				Expect(err).To(MatchError(ContainSubstring("the image doesn't have a quota applied")))
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

		Context("when the store path is not an XFS volume", func() {
			It("returns an error", func() {
				driver := overlayxfs.NewDriver("", "/tmp")
				_, err := driver.FetchStats(logger, spec.ImagePath)
				Expect(err).To(MatchError(ContainSubstring("cannot setup path for mount /tmp")))
			})
		})

		Context("when using a custom xfsprogs bin path", func() {
			It("will use binaries from that path", func() {
				driver := overlayxfs.NewDriver(XFSProgsPath, StorePath)
				imagePath := filepath.Join(StorePath, store.ImageDirName, fmt.Sprintf("random-id-%d", rand.Int()))
				Expect(os.Mkdir(imagePath, 0755)).To(Succeed())
				spec.ImagePath = imagePath

				Expect(driver.CreateImage(logger, spec)).To(Succeed())

				_, err := driver.FetchStats(logger, spec.ImagePath)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("quota usage output not as expected")))

				contents, err := ioutil.ReadFile(XFSQuotaCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - xfs_quota"))
			})
		})
	})
})

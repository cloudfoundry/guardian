package overlayxfs_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Driver", func() {
	var (
		driver     *overlayxfs.Driver
		logger     *lagertest.TestLogger
		storePath  string
		volumePath string
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("/mnt/xfs/", "store-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.Mkdir(filepath.Join(storePath, store.VolumesDirName), 0777)).To(Succeed())
		Expect(os.Mkdir(filepath.Join(storePath, store.ImageDirName), 0777)).To(Succeed())

		driver = overlayxfs.NewDriver(storePath)
		logger = lagertest.NewTestLogger("overlay+xfs")

		volumeID := randVolumeID()
		volumePath, err = driver.CreateVolume(logger, "parent-id", volumeID)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CreateImage", func() {
		var imagePath string

		BeforeEach(func() {
			imagePath = filepath.Join(storePath, store.ImageDirName, "random-id")
			Expect(os.Mkdir(imagePath, 0755)).To(Succeed())

			Expect(ioutil.WriteFile(filepath.Join(volumePath, "file-hello"), []byte("hello"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "file-bye"), []byte("bye"), 0700)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(volumePath, "a-folder"), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "a-folder", "folder-file"), []byte("in-a-folder"), 0755)).To(Succeed())
		})

		It("initializes the image path", func() {
			Expect(filepath.Join(imagePath, overlayxfs.UpperDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(imagePath, overlayxfs.WorkDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())

			Expect(driver.CreateImage(logger, volumePath, imagePath)).To(Succeed())

			Expect(filepath.Join(imagePath, overlayxfs.UpperDir)).To(BeADirectory())
			Expect(filepath.Join(imagePath, overlayxfs.WorkDir)).To(BeADirectory())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir)).To(BeADirectory())
		})

		It("creates a rootfs with the same files than the volume", func() {
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())
			Expect(driver.CreateImage(logger, volumePath, imagePath)).To(Succeed())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir)).To(BeADirectory())

			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir, "file-hello")).To(BeAnExistingFile())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir, "file-bye")).To(BeAnExistingFile())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir, "a-folder")).To(BeADirectory())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir, "a-folder", "folder-file")).To(BeAnExistingFile())
		})

		Context("when source folder does not exist", func() {
			It("returns an error", func() {
				err := driver.CreateImage(logger, "/not-real", imagePath)
				Expect(err).To(MatchError(ContainSubstring("source path does not exist")))
			})
		})

		Context("when image path folder doesn't exist", func() {
			It("returns an error", func() {
				err := driver.CreateImage(logger, volumePath, "/not-real")
				Expect(err).To(MatchError(ContainSubstring("image path does not exist")))
			})
		})

		Context("when creating the upper folder fails", func() {
			It("returns an error", func() {
				Expect(os.Mkdir(filepath.Join(imagePath, overlayxfs.UpperDir), 0755)).To(Succeed())
				err := driver.CreateImage(logger, volumePath, imagePath)
				Expect(err).To(MatchError(ContainSubstring("creating upperdir folder")))
			})
		})

		Context("when creating the workdir folder fails", func() {
			It("returns an error", func() {
				Expect(os.Mkdir(filepath.Join(imagePath, overlayxfs.WorkDir), 0755)).To(Succeed())
				err := driver.CreateImage(logger, volumePath, imagePath)
				Expect(err).To(MatchError(ContainSubstring("creating workdir folder")))
			})
		})

		Context("when creating the rootfs folder fails", func() {
			It("returns an error", func() {
				Expect(os.Mkdir(filepath.Join(imagePath, overlayxfs.RootfsDir), 0755)).To(Succeed())
				err := driver.CreateImage(logger, volumePath, imagePath)
				Expect(err).To(MatchError(ContainSubstring("creating rootfs folder")))
			})
		})
	})

	Describe("DestroyImage", func() {
		var imagePath string

		BeforeEach(func() {
			imagePath = filepath.Join(storePath, store.ImageDirName, "random-id")
			Expect(os.Mkdir(imagePath, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(volumePath, "file-hello"), []byte("hello"), 0755)).To(Succeed())
			Expect(driver.CreateImage(logger, volumePath, imagePath)).To(Succeed())
		})

		It("removes upper, work and rootfs dir from the image path", func() {
			Expect(filepath.Join(imagePath, overlayxfs.UpperDir)).To(BeADirectory())
			Expect(filepath.Join(imagePath, overlayxfs.WorkDir)).To(BeADirectory())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir)).To(BeADirectory())

			Expect(driver.DestroyImage(logger, imagePath)).To(Succeed())

			Expect(filepath.Join(imagePath, overlayxfs.UpperDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(imagePath, overlayxfs.WorkDir)).ToNot(BeAnExistingFile())
			Expect(filepath.Join(imagePath, overlayxfs.RootfsDir)).ToNot(BeAnExistingFile())
		})

		Context("when it fails unmount the rootfs", func() {
			It("returns an error", func() {
				Expect(syscall.Unmount(filepath.Join(imagePath, overlayxfs.RootfsDir), 0)).To(Succeed())

				err := driver.DestroyImage(logger, imagePath)
				Expect(err).To(MatchError(ContainSubstring("unmounting rootfs folder")))
			})
		})
	})
})

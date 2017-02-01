package image_cloner_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/groot"
	imageClonerpkg "code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/grootfs/store/image_cloner/image_clonerfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Image", func() {
	var (
		logger      lager.Logger
		storePath   string
		imagesPath  string
		imageCloner *imageClonerpkg.ImageCloner

		fakeImageDriver *image_clonerfakes.FakeImageDriver
	)

	BeforeEach(func() {
		var err error
		fakeImageDriver = new(image_clonerfakes.FakeImageDriver)

		fakeImageDriver.CreateImageStub = func(_ lager.Logger, from, to string) error {
			return os.Mkdir(to, 0777)
		}

		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		imagesPath = filepath.Join(storePath, "images")

		Expect(os.Mkdir(imagesPath, 0777)).To(Succeed())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-bunlder")
		imageCloner = imageClonerpkg.NewImageCloner(fakeImageDriver, storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("Create", func() {
		Describe("image ownership", func() {
			It("will change the ownership of all artifacts it creates", func() {
				uid := 2525
				gid := 2525

				image, err := imageCloner.Create(logger, groot.ImageSpec{
					ID:       "some-id",
					OwnerUID: uid,
					OwnerGID: gid,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(image.Path).To(BeADirectory())

				imagePath, err := os.Stat(image.Path)
				Expect(err).NotTo(HaveOccurred())
				Expect(imagePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
				Expect(imagePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(gid)))

				rootfsPath, err := os.Stat(image.RootFSPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(gid)))

				imageJsonPath, err := os.Stat(filepath.Join(image.Path, "image.json"))
				Expect(err).NotTo(HaveOccurred())
				Expect(imageJsonPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(uid)))
				Expect(imageJsonPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(gid)))
			})
		})

		Context("when only owner UID is 0", func() {
			It("tries to enforce ownership", func() {
				image, err := imageCloner.Create(logger, groot.ImageSpec{
					ID:       "some-id",
					OwnerUID: 0,
					OwnerGID: 10000,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(image.Path).To(BeADirectory())

				imagePath, err := os.Stat(image.Path)
				Expect(err).NotTo(HaveOccurred())
				Expect(imagePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
				Expect(imagePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10000)))

				rootfsPath, err := os.Stat(image.RootFSPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10000)))

				imageJsonPath, err := os.Stat(filepath.Join(image.Path, "image.json"))
				Expect(err).NotTo(HaveOccurred())
				Expect(imageJsonPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
				Expect(imageJsonPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(10000)))
			})
		})

		Context("when only owner GID is 0", func() {
			It("tries to enforce ownership", func() {
				image, err := imageCloner.Create(logger, groot.ImageSpec{
					ID:       "some-id",
					OwnerUID: 50000,
					OwnerGID: 0,
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(image.Path).To(BeADirectory())

				imagePath, err := os.Stat(image.Path)
				Expect(err).NotTo(HaveOccurred())
				Expect(imagePath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(50000)))
				Expect(imagePath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))

				rootfsPath, err := os.Stat(image.RootFSPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(50000)))
				Expect(rootfsPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))

				imageJsonPath, err := os.Stat(filepath.Join(image.Path, "image.json"))
				Expect(err).NotTo(HaveOccurred())
				Expect(imageJsonPath.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(50000)))
				Expect(imageJsonPath.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
			})
		})
	})
})

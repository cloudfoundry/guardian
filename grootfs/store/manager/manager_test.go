package manager_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/base_image_puller/base_image_pullerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/image_cloner/image_clonerfakes"
	managerpkg "code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/grootfs/store/storefakes"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var (
		imgDriver *image_clonerfakes.FakeImageDriver
		volDriver *base_image_pullerfakes.FakeVolumeDriver
		strDriver *storefakes.FakeStoreDriver
		locksmith *grootfakes.FakeLocksmith
		manager   *managerpkg.Manager
		storePath string
		logger    *lagertest.TestLogger
	)

	BeforeEach(func() {
		imgDriver = new(image_clonerfakes.FakeImageDriver)
		volDriver = new(base_image_pullerfakes.FakeVolumeDriver)
		strDriver = new(storefakes.FakeStoreDriver)
		locksmith = new(grootfakes.FakeLocksmith)

		logger = lagertest.NewTestLogger("store-manager")
	})

	AfterEach(func() {
		os.RemoveAll(storePath)
	})

	JustBeforeEach(func() {
		manager = managerpkg.New(storePath, locksmith, volDriver, imgDriver, strDriver)
	})

	Describe("Init", func() {
		BeforeEach(func() {
			storePath = filepath.Join(os.TempDir(), fmt.Sprintf("init-store-%d", GinkgoParallelNode()))
		})

		It("creates the store path folder", func() {
			Expect(storePath).ToNot(BeAnExistingFile())
			Expect(manager.InitStore(logger)).To(Succeed())
			Expect(storePath).To(BeADirectory())
		})

		It("validates the store path parent with the store driver", func() {
			Expect(manager.InitStore(logger)).To(Succeed())
			Expect(strDriver.ValidateFileSystemCallCount()).To(Equal(1))
		})

		Context("when the store path already exists", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(storePath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				err := manager.InitStore(logger)
				Expect(err).To(MatchError(ContainSubstring("store already initialized")))
			})
		})

		Context("when the store driver validation fails", func() {
			BeforeEach(func() {
				strDriver.ValidateFileSystemReturns(errors.New("not possible"))
			})

			It("returns an error", func() {
				err := manager.InitStore(logger)
				Expect(err).To(MatchError(ContainSubstring("not possible")))
			})
		})
	})

	Describe("DeleteStore", func() {
		var (
			imagesPath  string
			volumesPath string
		)

		BeforeEach(func() {
			var err error
			storePath, err = ioutil.TempDir("", "store-path")
			Expect(err).NotTo(HaveOccurred())

			imagesPath = filepath.Join(storePath, store.ImageDirName)
			Expect(os.Mkdir(imagesPath, 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(imagesPath, "img-1"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(imagesPath, "img-2"), 0755)).To(Succeed())

			volumesPath = filepath.Join(storePath, store.VolumesDirName)
			Expect(os.Mkdir(volumesPath, 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(volumesPath, "vol-1"), 0755)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(volumesPath, "vol-2"), 0755)).To(Succeed())
		})

		It("uses the image driver to delete all images in the store path", func() {
			Expect(manager.DeleteStore(logger)).To(Succeed())

			Expect(imgDriver.DestroyImageCallCount()).To(Equal(2))

			_, imageId := imgDriver.DestroyImageArgsForCall(0)
			Expect(imageId).To(Equal(filepath.Join(imagesPath, "img-1")))

			_, imageId = imgDriver.DestroyImageArgsForCall(1)
			Expect(imageId).To(Equal(filepath.Join(imagesPath, "img-2")))
		})

		It("uses the volume driver to delete all volumes in the store path", func() {
			Expect(manager.DeleteStore(logger)).To(Succeed())

			Expect(volDriver.DestroyVolumeCallCount()).To(Equal(2))

			_, volId := volDriver.DestroyVolumeArgsForCall(0)
			Expect(volId).To(Equal("vol-1"))

			_, volId = volDriver.DestroyVolumeArgsForCall(1)
			Expect(volId).To(Equal("vol-2"))
		})

		It("requests a lock", func() {
			Expect(manager.DeleteStore(logger)).To(Succeed())
			Expect(locksmith.LockCallCount()).To(Equal(1))
			Expect(locksmith.UnlockCallCount()).To(Equal(1))

			lockKey := locksmith.LockArgsForCall(0)
			Expect(lockKey).To(Equal(groot.GlobalLockKey))
		})

		It("deletes the store path", func() {
			Expect(storePath).To(BeAnExistingFile())
			Expect(manager.DeleteStore(logger)).To(Succeed())
			Expect(storePath).ToNot(BeAnExistingFile())
		})

		Context("when image driver fails to delete an image", func() {
			BeforeEach(func() {
				imgDriver.DestroyImageReturns(errors.New("failed to delete"))
			})

			It("returns an error", func() {
				err := manager.DeleteStore(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to delete")))
			})
		})

		Context("when volume driver fails to delete a volume", func() {
			BeforeEach(func() {
				volDriver.DestroyVolumeReturns(errors.New("failed to delete"))
			})

			It("returns an error", func() {
				err := manager.DeleteStore(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to delete")))
			})
		})

		Context("when the locksmith fails to lock", func() {
			BeforeEach(func() {
				locksmith.LockReturns(nil, errors.New("cant do it"))
			})

			It("returns an error", func() {
				err := manager.DeleteStore(logger)
				Expect(err).To(MatchError(ContainSubstring("cant do it")))
			})
		})

		Context("when the locksmith fails to unlock", func() {
			BeforeEach(func() {
				locksmith.UnlockReturns(errors.New("cant do it"))
			})

			It("doesn't fail", func() {
				Expect(manager.DeleteStore(logger)).To(Succeed())
			})
		})
	})
})

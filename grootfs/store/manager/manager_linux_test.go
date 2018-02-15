package manager_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/base_image_puller/base_image_pullerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/image_cloner/image_clonerfakes"
	managerpkg "code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/grootfs/store/manager/managerfakes"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Manager", func() {
	var (
		originalTmpDir string

		imgDriver   *image_clonerfakes.FakeImageDriver
		volDriver   *base_image_pullerfakes.FakeVolumeDriver
		storeDriver *managerfakes.FakeStoreDriver
		locksmith   *grootfakes.FakeLocksmith
		manager     *managerpkg.Manager
		storePath   string
		logger      *lagertest.TestLogger
		spec        managerpkg.InitSpec
		namespacer  *managerfakes.FakeStoreNamespacer
	)

	BeforeEach(func() {
		originalTmpDir = os.TempDir()

		imgDriver = new(image_clonerfakes.FakeImageDriver)
		volDriver = new(base_image_pullerfakes.FakeVolumeDriver)
		storeDriver = new(managerfakes.FakeStoreDriver)
		locksmith = new(grootfakes.FakeLocksmith)
		namespacer = new(managerfakes.FakeStoreNamespacer)

		logger = lagertest.NewTestLogger("store-manager")

		spec = managerpkg.InitSpec{}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
		Expect(os.Setenv("TMPDIR", originalTmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		manager = managerpkg.New(storePath, namespacer, volDriver, imgDriver, storeDriver)
	})

	Describe("InitStore", func() {
		BeforeEach(func() {
			storePath = filepath.Join(os.TempDir(), fmt.Sprintf("init-store-%d", GinkgoParallelNode()))
		})

		It("creates the store path folder", func() {
			Expect(storePath).ToNot(BeAnExistingFile())
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			Expect(storePath).To(BeADirectory())
		})

		It("sets the caller user as the owner of the store", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			stat, err := os.Stat(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
			Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
		})

		Describe("Path validation", func() {
			Context("when the store path folder exists", func() {
				BeforeEach(func() {
					Expect(os.MkdirAll(storePath, 0700)).To(Succeed())
				})

				It("validates the store path with the store driver", func() {
					Expect(manager.InitStore(logger, spec)).To(Succeed())
					Expect(storeDriver.ValidateFileSystemCallCount()).To(Equal(1))

					_, path := storeDriver.ValidateFileSystemArgsForCall(0)
					Expect(path).To(Equal(storePath))
				})
			})

			Context("when the store path folder doesn't exist", func() {
				It("validates the store path parent with the store driver", func() {
					Expect(manager.InitStore(logger, spec)).To(Succeed())
					Expect(storeDriver.ValidateFileSystemCallCount()).To(Equal(1))

					_, path := storeDriver.ValidateFileSystemArgsForCall(0)
					Expect(path).To(Equal(filepath.Dir(storePath)))
				})
			})
		})

		It("creates the correct internal structure", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())

			Expect(filepath.Join(storePath, "images")).To(BeADirectory())
			Expect(filepath.Join(storePath, "volumes")).To(BeADirectory())
			Expect(filepath.Join(storePath, "tmp")).To(BeADirectory())
			Expect(filepath.Join(storePath, "locks")).To(BeADirectory())
			Expect(filepath.Join(storePath, "meta")).To(BeADirectory())
			Expect(filepath.Join(storePath, "meta", "dependencies")).To(BeADirectory())
		})

		It("calls the namespace writer to set the metadata", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			Expect(namespacer.ApplyMappingsCallCount()).To(Equal(1))

			uidMappings, gidMappings := namespacer.ApplyMappingsArgsForCall(0)
			Expect(uidMappings).To(BeEmpty())
			Expect(gidMappings).To(BeEmpty())
		})

		It("calls the store driver to configure the store", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			Expect(storeDriver.ConfigureStoreCallCount()).To(Equal(1))

			_, storePathArg, uidArg, gidArg := storeDriver.ConfigureStoreArgsForCall(0)
			Expect(storePathArg).To(Equal(storePath))
			Expect(uidArg).To(Equal(0))
			Expect(gidArg).To(Equal(0))
		})

		It("chmods the storePath to 700", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())

			stat, err := os.Stat(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
		})

		Context("when store driver configuration fails", func() {
			It("returns an error", func() {
				storeDriver.ConfigureStoreReturns(errors.New("configuration failed"))
				err := manager.InitStore(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("running filesystem-specific configuration")))
			})
		})

		Context("when any internal directory already exists", func() {
			It("succeeds", func() {
				Expect(os.MkdirAll(filepath.Join(storePath, "volumes"), 0700)).To(Succeed())
				Expect(manager.InitStore(logger, spec)).To(Succeed())
			})
		})
		Context("when the namespacer fails to create", func() {
			BeforeEach(func() {
				namespacer.ApplyMappingsReturns(errors.New("failed to create"))
			})

			It("returns an error", func() {
				err := manager.InitStore(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("failed to create")))
			})
		})

		Context("when store driver filesystem verification fails", func() {
			It("returns an error", func() {
				storeDriver.ValidateFileSystemReturns(errors.New("not a valid filesystem"))
				err := manager.InitStore(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("not a valid filesystem")))
			})
		})

		Context("when id mappings are provided", func() {
			var (
				uidMappings []groot.IDMappingSpec
				gidMappings []groot.IDMappingSpec
			)

			BeforeEach(func() {
				uidMappings = []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10000, NamespaceID: 1, Size: 10},
					groot.IDMappingSpec{HostID: int(GrootUID), NamespaceID: 0, Size: 1},
				}
				spec.UIDMappings = uidMappings

				gidMappings = []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 10000, NamespaceID: 1, Size: 10},
					groot.IDMappingSpec{HostID: int(GrootGID), NamespaceID: 0, Size: 1},
				}
				spec.GIDMappings = gidMappings
			})

			It("sets the root mapping as the owner of the store", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				stat, err := os.Stat(storePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(GrootUID))
				Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(GrootGID))
			})

			It("calls the namespace writer to set the metadata", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(namespacer.ApplyMappingsCallCount()).To(Equal(1))

				uidMappingsArg, gidMappingsArg := namespacer.ApplyMappingsArgsForCall(0)
				Expect(uidMappingsArg).To(Equal(uidMappings))
				Expect(gidMappingsArg).To(Equal(gidMappings))
			})

			It("calls the store driver to configure the store", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(storeDriver.ConfigureStoreCallCount()).To(Equal(1))

				_, storePathArg, uidArg, gidArg := storeDriver.ConfigureStoreArgsForCall(0)
				Expect(storePathArg).To(Equal(storePath))
				Expect(uidArg).To(Equal(int(GrootUID)))
				Expect(gidArg).To(Equal(int(GrootGID)))
			})

			Context("when the root mapping is not present", func() {
				BeforeEach(func() {
					spec.UIDMappings = []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 10000, NamespaceID: 1, Size: 10},
					}
					spec.GIDMappings = []groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 10000, NamespaceID: 1, Size: 10},
					}
				})

				It("sets caller user as the owner of the store", func() {
					Expect(manager.InitStore(logger, spec)).To(Succeed())
					stat, err := os.Stat(storePath)
					Expect(err).NotTo(HaveOccurred())
					Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(0)))
					Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(0)))
				})
			})
		})

		Context("when store size is provided", func() {
			var backingStoreFile string

			BeforeEach(func() {
				spec.StoreSizeBytes = 1024 * 1024 * 500
				backingStoreFile = fmt.Sprintf("%s.backing-store", storePath)
				storeDriver.ValidateFileSystemReturns(errors.New("not initilized yet"))
			})

			AfterEach(func() {
				Expect(os.RemoveAll(backingStoreFile)).To(Succeed())
			})

			It("creates a backing store file", func() {
				Expect(backingStoreFile).ToNot(BeAnExistingFile())
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(backingStoreFile).To(BeAnExistingFile())
			})

			It("truncates the backing store file with the right size", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())

				stats, err := os.Stat(backingStoreFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats.Size()).To(Equal(int64(1024 * 1024 * 500)))
			})

			It("uses the storedriver to initialize the filesystem", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(storeDriver.InitFilesystemCallCount()).To(Equal(1), "store driver was not called")

				_, filesystemPathArg, storePathArg := storeDriver.InitFilesystemArgsForCall(0)
				Expect(filesystemPathArg).To(Equal(backingStoreFile))
				Expect(storePathArg).To(Equal(storePath))
			})

			It("validates the store path with the store driver", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(storeDriver.ValidateFileSystemCallCount()).To(Equal(1))

				_, path := storeDriver.ValidateFileSystemArgsForCall(0)
				Expect(path).To(Equal(storePath))
			})

			Context("when the store is considered valid", func() {
				BeforeEach(func() {
					storeDriver.ValidateFileSystemReturns(nil)
				})

				It("doesn't call the storedriver to initialize it", func() {
					Expect(manager.InitStore(logger, spec)).To(Succeed())
					Expect(storeDriver.InitFilesystemCallCount()).To(BeZero())
				})
			})

			Context("when the backingstore file already exists", func() {
				BeforeEach(func() {
					Expect(ioutil.WriteFile(backingStoreFile, []byte{}, 0700)).To(Succeed())
				})

				It("succeeds", func() {
					Expect(manager.InitStore(logger, spec)).To(Succeed())
				})

			})

			Context("when the store directory already exists", func() {
				BeforeEach(func() {
					Expect(os.MkdirAll(storePath, 0755)).To(Succeed())
				})

				It("succeeds", func() {
					err := manager.InitStore(logger, spec)
					Expect(err).To(Succeed())
				})
			})

			Context("when the store driver fails to initialize the filesystem", func() {
				BeforeEach(func() {
					storeDriver.InitFilesystemReturns(errors.New("failed!"))
				})

				It("returns an error", func() {
					err := manager.InitStore(logger, spec)
					Expect(err).To(MatchError(ContainSubstring("failed!")))
				})
			})
		})

		Context("when the store driver validation fails", func() {
			BeforeEach(func() {
				storeDriver.ValidateFileSystemReturns(errors.New("not possible"))
			})

			It("returns an error", func() {
				err := manager.InitStore(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("not possible")))
			})
		})

		Context("when the store is already initialized", func() {
			JustBeforeEach(func() {
				spec.StoreSizeBytes = 10000000000000
				storeDriver.ValidateFileSystemReturns(nil)
			})

			It("logs the event", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(logger.Buffer()).To(gbytes.Say("store-already-initialized"))
			})

			It("doesn't try to recreate/mount the backing store", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(storeDriver.InitFilesystemCallCount()).To(Equal(0))
			})
		})
	})

	Describe("IsStoreInitialized", func() {
		BeforeEach(func() {
			var err error
			storePath, err = ioutil.TempDir("", "init-store")
			Expect(err).NotTo(HaveOccurred())

			namespacer.ApplyMappingsStub = func(_, _ []groot.IDMappingSpec) error {
				return ioutil.WriteFile(filepath.Join(storePath, store.MetaDirName, groot.NamespaceFilename), []byte{}, 0666)
			}
		})

		Context("when the store is initialized", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
			})

			It("returns true", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeTrue())
			})
		})

		Context("when the store is missing the images dir", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(os.RemoveAll(filepath.Join(storePath, store.ImageDirName))).To(Succeed())
			})

			It("returns false", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeFalse())
			})
		})

		Context("when the store is missing the volumes dir", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(os.RemoveAll(filepath.Join(storePath, store.VolumesDirName))).To(Succeed())
			})

			It("returns false", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeFalse())
			})
		})

		Context("when the store is missing the meta dir", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(os.RemoveAll(filepath.Join(storePath, store.MetaDirName))).To(Succeed())
			})

			It("returns false", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeFalse())
			})
		})

		Context("when the store is missing the locks dir", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(os.RemoveAll(filepath.Join(storePath, store.LocksDirName))).To(Succeed())
			})

			It("returns false", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeFalse())
			})
		})

		Context("when the store is missing the temp dir", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(os.RemoveAll(filepath.Join(storePath, store.TempDirName))).To(Succeed())
			})

			It("returns false", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeFalse())
			})
		})

		Context("when the store is missing the dependencies dir", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(os.RemoveAll(filepath.Join(storePath, store.MetaDirName, "dependencies"))).To(Succeed())
			})

			It("returns false", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeFalse())
			})
		})

		Context("when the store is missing the namepsace.json", func() {
			JustBeforeEach(func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(os.RemoveAll(filepath.Join(storePath, store.MetaDirName, groot.NamespaceFilename))).To(Succeed())
			})

			It("returns false", func() {
				Expect(manager.IsStoreInitialized(logger)).To(BeFalse())
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

		It("uses the store driver to deinitialise the store", func() {
			Expect(manager.DeleteStore(logger)).To(Succeed())

			Expect(storeDriver.DeInitFilesystemCallCount()).To(Equal(1))

			_, path := storeDriver.DeInitFilesystemArgsForCall(0)
			Expect(path).To(Equal(storePath))
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

		Context("when store driver fails to deinitialise the store", func() {
			BeforeEach(func() {
				storeDriver.DeInitFilesystemReturns(errors.New("failed to deinitialise"))
			})

			It("returns an error", func() {
				err := manager.DeleteStore(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to deinitialise")))
			})
		})

		Context("when the store path does not exist", func() {
			BeforeEach(func() {
				storePath = ""
			})

			It("fails", func() {
				Expect(manager.DeleteStore(logger)).To(Succeed())
			})
		})
	})
})

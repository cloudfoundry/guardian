package manager_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"code.cloudfoundry.org/grootfs/base_image_puller/base_image_pullerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/image_manager/image_managerfakes"
	managerpkg "code.cloudfoundry.org/grootfs/store/manager"
	"code.cloudfoundry.org/grootfs/store/manager/managerfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"golang.org/x/sys/unix"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Manager", func() {
	var (
		originalTmpDir string

		imgDriver   *image_managerfakes.FakeImageDriver
		volDriver   *base_image_pullerfakes.FakeVolumeDriver
		storeDriver *managerfakes.FakeStoreDriver
		manager     *managerpkg.Manager
		storePath   string
		logger      *lagertest.TestLogger
		spec        managerpkg.InitSpec
		namespacer  *managerfakes.FakeStoreNamespacer
		locksmith   *grootfakes.FakeLocksmith
		grootfsPath string
	)

	BeforeEach(func() {
		originalTmpDir = os.TempDir()
		grootfsPath = filepath.Join(os.TempDir(), fmt.Sprintf("grootfs-%d", GinkgoParallelProcess()))

		imgDriver = new(image_managerfakes.FakeImageDriver)
		volDriver = new(base_image_pullerfakes.FakeVolumeDriver)
		storeDriver = new(managerfakes.FakeStoreDriver)
		namespacer = new(managerfakes.FakeStoreNamespacer)
		locksmith = new(grootfakes.FakeLocksmith)

		logger = lagertest.NewTestLogger("store-manager")

		spec = managerpkg.InitSpec{}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(grootfsPath)).To(Succeed())
		Expect(os.Setenv("TMPDIR", originalTmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		manager = managerpkg.New(storePath, namespacer, volDriver, imgDriver, storeDriver, locksmith)
	})

	Describe("InitStore", func() {
		BeforeEach(func() {
			storePath = filepath.Join(grootfsPath, "init-store")
		})

		Describe("Critical Section", func() {
			var (
				lockfile    *os.File
				lockMutex   *sync.Mutex
				lockChannel chan bool
			)

			BeforeEach(func() {
				lockMutex = new(sync.Mutex)
				lockChannel = make(chan bool)

				locksmith.LockStub = func(key string) (*os.File, error) {
					lockMutex.Lock()
					return lockfile, nil
				}

				locksmith.UnlockStub = func(lockFile *os.File) error {
					lockMutex.Unlock()
					return nil
				}
			})

			It("uses locksmith to provide a locking mechanism", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(locksmith.LockCallCount()).To(Equal(1))

				Expect(locksmith.UnlockCallCount()).To(Equal(1))
				actualLockfile := locksmith.UnlockArgsForCall(0)
				Expect(lockfile).To(Equal(actualLockfile))
			})

			It("serialises the critical section", func() {
				storeDriver.ValidateFileSystemStub = func(_ lager.Logger, _ string) error {
					<-lockChannel
					return nil
				}

				// Start two concurrent InitStore funcs
				// In the BeforeEach, we have set ValidateFileystem to block to ensure both
				// InitStore funcs try to enter the critical section
				wg := &sync.WaitGroup{}
				wg.Add(2)

				go func() {
					defer func() {
						GinkgoRecover()
						wg.Done()
					}()
					Expect(manager.InitStore(logger, spec)).To(Succeed())
				}()

				go func() {
					defer func() {
						GinkgoRecover()
						wg.Done()
					}()
					Expect(manager.InitStore(logger, spec)).To(Succeed())
				}()

				// Check both InitStores have tried to obtain the lock
				Eventually(locksmith.LockCallCount).Should(Equal(2))

				// Check that only one InitStore has obtained the lock
				Consistently(storeDriver.ValidateFileSystemCallCount).Should(Equal(1))

				// Release the first InitStore and check the other obtains the lock and continues
				lockChannel <- true
				Eventually(storeDriver.ValidateFileSystemCallCount).Should(Equal(2))

				// Release the second InitStore
				close(lockChannel)

				// Wait for both InitStores to complete to avoid go routine pollution
				wg.Wait()
			})

			It("uses a clear lock name", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(locksmith.LockArgsForCall(0)).To(Equal("init-store"))
			})

			Context("when locking fails", func() {
				BeforeEach(func() {
					locksmith.LockReturns(nil, errors.New("lock-error"))
				})

				It("bubbles up the error", func() {
					Expect(manager.InitStore(logger, spec)).To(MatchError(ContainSubstring("lock-error")))
				})
			})

			Context("when something fails after locking", func() {
				BeforeEach(func() {
					namespacer.ApplyMappingsReturns(errors.New("apply-mappings-error"))
				})

				It("still unlocks", func() {
					Expect(manager.InitStore(logger, spec)).To(MatchError(ContainSubstring("apply-mappings-error")))
					Expect(locksmith.UnlockCallCount()).To(Equal(1))
				})

				Context("when the unlock also fails", func() {
					BeforeEach(func() {
						locksmith.UnlockReturns(errors.New("unlock-error"))
					})

					It("bubbles up the errors", func() {
						err := manager.InitStore(logger, spec)
						Expect(err).To(MatchError(ContainSubstring("unlock-error")))
						Expect(err).To(MatchError(ContainSubstring("apply-mappings-error")))
					})
				})
			})
		})

		It("sets the caller user as the owner of the store", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			var stat unix.Stat_t
			Expect(unix.Stat(storePath, &stat)).To(Succeed())
			Expect(stat.Uid).To(Equal(uint32(0)))
			Expect(stat.Gid).To(Equal(uint32(0)))
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

			_, storePathArg, backingStorePathArg, uidArg, gidArg := storeDriver.ConfigureStoreArgsForCall(0)
			Expect(storePathArg).To(Equal(storePath))
			Expect(backingStorePathArg).To(Equal(storePath + ".backing-store"))
			Expect(uidArg).To(Equal(0))
			Expect(gidArg).To(Equal(0))
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
				var stat unix.Stat_t
				Expect(unix.Stat(storePath, &stat)).To(Succeed())
				Expect(stat.Uid).To(Equal(GrootUID))
				Expect(stat.Gid).To(Equal(GrootGID))
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

				_, storePathArg, backingStorePathArg, uidArg, gidArg := storeDriver.ConfigureStoreArgsForCall(0)
				Expect(storePathArg).To(Equal(storePath))
				Expect(backingStorePathArg).To(Equal(storePath + ".backing-store"))
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
					var stat unix.Stat_t
					Expect(unix.Stat(storePath, &stat)).To(Succeed())
					Expect(stat.Uid).To(Equal(uint32(0)))
					Expect(stat.Gid).To(Equal(uint32(0)))
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

			It("creates the storePath with 0700 permissions", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				stat, err := os.Stat(storePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
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
					Expect(os.MkdirAll(storePath, 0755)).To(Succeed())
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
					Expect(manager.InitStore(logger, spec)).To(Succeed())
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

		When("the backing store exists", func() {
			BeforeEach(func() {
				spec.StoreSizeBytes = 10000000000000
				Expect(os.MkdirAll(storePath, 0755)).To(Succeed())
				Expect(ioutil.WriteFile(fmt.Sprintf("%s.backing-store", storePath), []byte{}, 0777)).To(Succeed())
			})

			It("mounts it", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(storeDriver.MountFilesystemCallCount()).To(Equal(1))
				_, actualBackingStorePath, actualStorePath := storeDriver.MountFilesystemArgsForCall(0)
				Expect(actualBackingStorePath).To(Equal(fmt.Sprintf("%s.backing-store", storePath)))
				Expect(actualStorePath).To(Equal(storePath))
			})

			When("the store path is already a mountpoint", func() {
				BeforeEach(func() {
					Expect(unix.Mount(storePath, storePath, "bind", unix.MS_BIND, "")).To(Succeed())
				})

				AfterEach(func() {
					Expect(unix.Unmount(storePath, 0)).To(Succeed())

				})

				It("succeeds", func() {
					Expect(manager.InitStore(logger, spec)).To(Succeed())
				})

				It("does not remount it", func() {
					Expect(storeDriver.MountFilesystemCallCount()).To(Equal(0))
				})
			})

			When("mounting the backing store fails for any other reason", func() {
				BeforeEach(func() {
					storeDriver.MountFilesystemReturns(errors.New("mount-failure"))
					// mounting failed => validation fails (due to e.g. corrupted backing store)
					storeDriver.ValidateFileSystemReturns(errors.New("validation-error"))
				})

				It("reinits the store", func() {
					Expect(manager.InitStore(logger, spec)).To(Succeed())
					Expect(storeDriver.InitFilesystemCallCount()).To(Equal(1))
				})
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

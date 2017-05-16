package manager_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
)

var _ = Describe("Manager", func() {
	var (
		originalTmpDir string

		imgDriver  *image_clonerfakes.FakeImageDriver
		volDriver  *base_image_pullerfakes.FakeVolumeDriver
		strDriver  *managerfakes.FakeStoreDriver
		locksmith  *grootfakes.FakeLocksmith
		manager    *managerpkg.Manager
		storePath  string
		logger     *lagertest.TestLogger
		spec       managerpkg.InitSpec
		namespacer *managerfakes.FakeNamespaceWriter
	)

	BeforeEach(func() {
		originalTmpDir = os.TempDir()

		imgDriver = new(image_clonerfakes.FakeImageDriver)
		volDriver = new(base_image_pullerfakes.FakeVolumeDriver)
		strDriver = new(managerfakes.FakeStoreDriver)
		locksmith = new(grootfakes.FakeLocksmith)
		namespacer = new(managerfakes.FakeNamespaceWriter)

		logger = lagertest.NewTestLogger("store-manager")

		spec = managerpkg.InitSpec{
			NamespaceWriter: namespacer,
		}
	})

	AfterEach(func() {
		os.RemoveAll(storePath)
		Expect(os.Setenv("TMPDIR", originalTmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		manager = managerpkg.New(storePath, locksmith, volDriver, imgDriver, strDriver)
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

		It("validates the store path parent with the store driver", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			Expect(strDriver.ValidateFileSystemCallCount()).To(Equal(1))
		})

		It("creates the metadata directory", func() {
			Expect(filepath.Join(storePath, store.MetaDirName)).ToNot(BeAnExistingFile())
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			Expect(filepath.Join(storePath, store.MetaDirName)).To(BeADirectory())
		})

		It("calls the namespace writer to set the metadata", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			Expect(namespacer.WriteCallCount()).To(Equal(1))

			storePathArg, uidMappings, gidMappings := namespacer.WriteArgsForCall(0)
			Expect(storePathArg).To(Equal(storePath))
			Expect(uidMappings).To(BeEmpty())
			Expect(gidMappings).To(BeEmpty())
		})

		It("calls the store driver to configure the store", func() {
			Expect(manager.InitStore(logger, spec)).To(Succeed())
			Expect(strDriver.ConfigureStoreCallCount()).To(Equal(1))

			_, storePathArg, uidArg, gidArg := strDriver.ConfigureStoreArgsForCall(0)
			Expect(storePathArg).To(Equal(storePath))
			Expect(uidArg).To(Equal(0))
			Expect(gidArg).To(Equal(0))
		})

		Context("when the namespaceWriter fails", func() {
			BeforeEach(func() {
				namespacer.WriteReturns(errors.New("failed to create"))
			})

			It("returns an error", func() {
				err := manager.InitStore(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("failed to create")))
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
				Expect(namespacer.WriteCallCount()).To(Equal(1))

				storePathArg, uidMappingsArg, gidMappingsArg := namespacer.WriteArgsForCall(0)
				Expect(storePathArg).To(Equal(storePath))
				Expect(uidMappingsArg).To(Equal(uidMappings))
				Expect(gidMappingsArg).To(Equal(gidMappings))
			})

			It("calls the store driver to configure the store", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(strDriver.ConfigureStoreCallCount()).To(Equal(1))

				_, storePathArg, uidArg, gidArg := strDriver.ConfigureStoreArgsForCall(0)
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
			var backingstoreFile string

			BeforeEach(func() {
				spec.StoreSizeBytes = 1024 * 1024 * 500
				backingstoreFile = fmt.Sprintf("%s.backing-store", storePath)
			})

			AfterEach(func() {
				Expect(os.RemoveAll(backingstoreFile)).To(Succeed())
			})

			It("creates a backing store file", func() {
				Expect(backingstoreFile).ToNot(BeAnExistingFile())
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(backingstoreFile).To(BeAnExistingFile())
			})

			It("truncates the backing store file with the right size", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())

				stats, err := os.Stat(backingstoreFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats.Size()).To(Equal(int64(1024 * 1024 * 500)))
			})

			It("uses the storedriver to initialize the filesystem", func() {
				Expect(manager.InitStore(logger, spec)).To(Succeed())
				Expect(strDriver.InitFilesystemCallCount()).To(Equal(1), "store driver was not called")

				_, filesystemPathArg, storePathArg := strDriver.InitFilesystemArgsForCall(0)
				Expect(filesystemPathArg).To(Equal(backingstoreFile))
				Expect(storePathArg).To(Equal(storePath))
			})

			Context("when the store size is less than 200Mb", func() {
				BeforeEach(func() {
					spec.StoreSizeBytes = 1024 * 1024 * 199
				})

				It("returns an error", func() {
					err := manager.InitStore(logger, spec)
					Expect(err).To(MatchError(ContainSubstring("store size must be at least 200Mb")))
				})

				It("doesn't create the store folder", func() {
					Expect(storePath).ToNot(BeAnExistingFile())
					err := manager.InitStore(logger, spec)
					Expect(err).To(HaveOccurred())
					Expect(storePath).ToNot(BeAnExistingFile())
				})
			})

			Context("when the backingstore file already exits", func() {
				BeforeEach(func() {
					Expect(ioutil.WriteFile(backingstoreFile, []byte{}, 0700)).To(Succeed())
				})

				It("returns an error", func() {
					err := manager.InitStore(logger, spec)
					Expect(err).To(MatchError(ContainSubstring("backing store file already exists")))
				})
			})

			Context("when the storedrive fails to initialize the fs", func() {
				BeforeEach(func() {
					strDriver.InitFilesystemReturns(errors.New("failed!"))
				})

				It("returns an error", func() {
					err := manager.InitStore(logger, spec)
					Expect(err).To(MatchError(ContainSubstring("failed!")))
				})
			})
		})

		Context("when the store path already exists", func() {
			BeforeEach(func() {
				Expect(os.MkdirAll(storePath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				err := manager.InitStore(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("store already initialized")))
			})
		})

		Context("when the store driver validation fails", func() {
			BeforeEach(func() {
				strDriver.ValidateFileSystemReturns(errors.New("not possible"))
			})

			It("returns an error", func() {
				err := manager.InitStore(logger, spec)
				Expect(err).To(MatchError(ContainSubstring("not possible")))
			})
		})
	})

	Describe("ConfigureStore", func() {
		var (
			currentUID int
			currentGID int
		)

		BeforeEach(func() {
			tempDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			currentUID = os.Getuid()
			currentGID = os.Getgid()
			storePath = path.Join(tempDir, fmt.Sprintf("store-%d", GinkgoParallelNode()))

			logger = lagertest.NewTestLogger("store-configurer")
		})

		AfterEach(func() {
			Expect(os.RemoveAll(path.Dir(storePath))).To(Succeed())
		})

		It("creates the store directory", func() {
			Expect(storePath).ToNot(BeAnExistingFile())
			Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
			Expect(storePath).To(BeADirectory())
		})

		It("creates the correct internal structure", func() {
			Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())

			Expect(filepath.Join(storePath, "images")).To(BeADirectory())
			Expect(filepath.Join(storePath, "cache")).To(BeADirectory())
			Expect(filepath.Join(storePath, "volumes")).To(BeADirectory())
			Expect(filepath.Join(storePath, "tmp")).To(BeADirectory())
			Expect(filepath.Join(storePath, "locks")).To(BeADirectory())
			Expect(filepath.Join(storePath, "meta")).To(BeADirectory())
			Expect(filepath.Join(storePath, "meta", "dependencies")).To(BeADirectory())
		})

		It("creates tmp files into TMPDIR inside storePath", func() {
			Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
			file, _ := ioutil.TempFile("", "")
			Expect(filepath.Join(storePath, store.TempDirName, filepath.Base(file.Name()))).To(BeAnExistingFile())
		})

		It("chmods the storePath to 700", func() {
			Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())

			stat, err := os.Stat(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Mode().Perm()).To(Equal(os.FileMode(0700)))
		})

		It("chowns the storePath to the owner UID/GID", func() {
			Expect(manager.ConfigureStore(logger, 1, 2)).To(Succeed())

			stat, err := os.Stat(storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stat.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(1)))
			Expect(stat.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(2)))
		})

		It("calls the store driver for configuration", func() {
			Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
			Expect(strDriver.ConfigureStoreCallCount()).To(Equal(1))

			_, path, ownerUID, ownerGID := strDriver.ConfigureStoreArgsForCall(0)
			Expect(path).To(Equal(storePath))
			Expect(ownerUID).To(Equal(currentUID))
			Expect(ownerGID).To(Equal(currentGID))
		})

		Context("when store driver configuration fails", func() {
			It("returns an error", func() {
				strDriver.ConfigureStoreReturns(errors.New("configuration failed"))
				err := manager.ConfigureStore(logger, currentUID, currentGID)
				Expect(err).To(MatchError(ContainSubstring("running filesystem-specific configuration")))
			})
		})

		It("calls the store driver for filesystem verification", func() {
			Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
			Expect(strDriver.ValidateFileSystemCallCount()).To(Equal(1))
			_, path := strDriver.ValidateFileSystemArgsForCall(0)
			Expect(path).To(Equal(storePath))
		})

		Context("when store driver filesystem verification fails", func() {
			It("returns an error", func() {
				strDriver.ValidateFileSystemReturns(errors.New("not a valid filesystem"))
				err := manager.ConfigureStore(logger, currentUID, currentGID)
				Expect(err).To(MatchError(ContainSubstring("not a valid filesystem")))
			})
		})

		It("doesn't fail on race conditions", func() {
			for i := 0; i < 50; i++ {
				storePath, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())
				manager = managerpkg.New(storePath, locksmith, volDriver, imgDriver, strDriver)
				start1 := make(chan bool)
				start2 := make(chan bool)

				go func() {
					defer GinkgoRecover()
					<-start1
					Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
					close(start1)
				}()

				go func() {
					defer GinkgoRecover()
					<-start2
					Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
					close(start2)
				}()

				start1 <- true
				start2 <- true

				Eventually(start1).Should(BeClosed())
				Eventually(start2).Should(BeClosed())
			}
		})

		Context("when the base directory does not exist", func() {
			BeforeEach(func() {
				storePath = "/does/not/exist"
			})

			It("returns an error", func() {
				Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(
					MatchError(ContainSubstring("making directory")),
				)
			})
		})

		Context("when the store already exists", func() {
			It("succeeds", func() {
				Expect(os.Mkdir(storePath, 0700)).To(Succeed())
				Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
			})

			Context("and it's a regular file", func() {
				It("returns an error", func() {
					Expect(ioutil.WriteFile(storePath, []byte("hello"), 0600)).To(Succeed())

					Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(
						MatchError(ContainSubstring("is not a directory")),
					)
				})
			})
		})

		Context("when any internal directory already exists", func() {
			It("succeeds", func() {
				Expect(os.MkdirAll(filepath.Join(storePath, "volumes"), 0700)).To(Succeed())
				Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(Succeed())
			})

			Context("and it's a regular file", func() {
				It("returns an error", func() {
					Expect(os.Mkdir(storePath, 0700)).To(Succeed())
					Expect(ioutil.WriteFile(filepath.Join(storePath, "volumes"), []byte("hello"), 0600)).To(Succeed())

					Expect(manager.ConfigureStore(logger, currentUID, currentGID)).To(
						MatchError(ContainSubstring("is not a directory")),
					)
				})
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

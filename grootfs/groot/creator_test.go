package groot_test

import (
	"errors"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Creator", func() {
	var (
		fakeImageCloner       *grootfakes.FakeImageCloner
		fakeBaseImagePuller   *grootfakes.FakeBaseImagePuller
		fakeLocksmith         *grootfakes.FakeLocksmith
		fakeRootFSConfigurer  *grootfakes.FakeRootFSConfigurer
		fakeDependencyManager *grootfakes.FakeDependencyManager
		fakeMetricsEmitter    *grootfakes.FakeMetricsEmitter
		fakeCleaner           *grootfakes.FakeCleaner
		fakeNamespaceChecker  *grootfakes.FakeNamespaceChecker
		lockFile              *os.File

		creator *groot.Creator
		logger  lager.Logger
	)

	BeforeEach(func() {
		fakeImageCloner = new(grootfakes.FakeImageCloner)
		fakeBaseImagePuller = new(grootfakes.FakeBaseImagePuller)
		fakeLocksmith = new(grootfakes.FakeLocksmith)
		fakeRootFSConfigurer = new(grootfakes.FakeRootFSConfigurer)
		fakeDependencyManager = new(grootfakes.FakeDependencyManager)
		fakeMetricsEmitter = new(grootfakes.FakeMetricsEmitter)
		fakeCleaner = new(grootfakes.FakeCleaner)
		fakeNamespaceChecker = new(grootfakes.FakeNamespaceChecker)

		var err error
		lockFile, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		fakeLocksmith.LockReturns(lockFile, nil)
		fakeNamespaceChecker.CheckReturns(true, nil)

		logger = lagertest.NewTestLogger("creator")

		fakeImageCloner.CreateReturns(groot.ImageInfo{
			Path:   "/path/to/images/123",
			Rootfs: "/path/to/images/123/rootfs",
		}, nil)

		creator = groot.IamCreator(
			fakeImageCloner, fakeBaseImagePuller, fakeLocksmith,
			fakeRootFSConfigurer, fakeDependencyManager, fakeMetricsEmitter,
			fakeCleaner, fakeNamespaceChecker)
	})

	AfterEach(func() {
		Expect(os.Remove(lockFile.Name())).To(Succeed())
	})

	Describe("Create", func() {
		It("acquires the global lock", func() {
			_, err := creator.Create(logger, groot.CreateSpec{
				BaseImage: "/path/to/image",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.LockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.LockArgsForCall(0)).To(Equal(groot.GlobalLockKey))
		})

		It("configures the store based on the mappings", func() {
			uidMappings := groot.IDMappingSpec{
				HostID:      1000,
				NamespaceID: 0,
				Size:        1,
			}

			gidMappings := groot.IDMappingSpec{
				HostID:      2000,
				NamespaceID: 0,
				Size:        1,
			}
			_, err := creator.Create(logger, groot.CreateSpec{
				BaseImage:   "/path/to/image",
				UIDMappings: []groot.IDMappingSpec{uidMappings},
				GIDMappings: []groot.IDMappingSpec{gidMappings},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeNamespaceChecker.CheckCallCount()).To(Equal(1))
			uids, gids := fakeNamespaceChecker.CheckArgsForCall(0)
			Expect(uids).To(ConsistOf(uidMappings))
			Expect(gids).To(ConsistOf(gidMappings))
		})

		Context("when clean up store is requested", func() {
			It("cleans the store", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage:                   "/path/to/image",
					CleanOnCreate:               true,
					CleanOnCreateIgnoreImages:   []string{"docker://my-image"},
					CleanOnCreateThresholdBytes: int64(250000),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeCleaner.CleanCallCount()).To(Equal(1))
				_, threshold, ignoredImages, acquireLock := fakeCleaner.CleanArgsForCall(0)
				Expect(threshold).To(Equal(int64(250000)))
				Expect(ignoredImages).To(ConsistOf([]string{"/path/to/image", "docker://my-image"}))
				Expect(acquireLock).To(BeFalse())
			})

			Context("and fails to clean up", func() {
				BeforeEach(func() {
					fakeCleaner.CleanReturns(true, errors.New("failed to clean up store"))
				})

				It("returns an error", func() {
					_, err := creator.Create(logger, groot.CreateSpec{
						BaseImage:     "/path/to/image",
						CleanOnCreate: true,
					})
					Expect(err).To(MatchError(ContainSubstring("failed to clean up store")))
				})
			})
		})

		It("pulls the image", func() {
			uidMappings := []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 2, NamespaceID: 0, Size: 1}}
			gidMappings := []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 3, NamespaceID: 0, Size: 1}}

			_, err := creator.Create(logger, groot.CreateSpec{
				BaseImage:   "/path/to/image",
				UIDMappings: uidMappings,
				GIDMappings: gidMappings,
			})
			Expect(err).NotTo(HaveOccurred())

			baseImageURL, err := url.Parse("/path/to/image")
			Expect(err).NotTo(HaveOccurred())
			_, imageSpec := fakeBaseImagePuller.PullArgsForCall(0)
			Expect(imageSpec.BaseImageSrc).To(Equal(baseImageURL))
			Expect(imageSpec.UIDMappings).To(Equal(uidMappings))
			Expect(imageSpec.GIDMappings).To(Equal(gidMappings))
			Expect(imageSpec.OwnerUID).To(Equal(2))
			Expect(imageSpec.OwnerGID).To(Equal(3))
		})

		It("makes an image", func() {
			baseImage := groot.BaseImage{
				ChainIDs: []string{"id-1", "id-2"},
				BaseImage: specsv1.Image{
					Author: "Groot",
				},
			}
			fakeBaseImagePuller.PullReturns(baseImage, nil)

			uidMappings := []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 50, NamespaceID: 0, Size: 1}}
			gidMappings := []groot.IDMappingSpec{groot.IDMappingSpec{HostID: 60, NamespaceID: 0, Size: 1}}
			_, err := creator.Create(logger, groot.CreateSpec{
				ID:          "some-id",
				BaseImage:   "/path/to/image",
				UIDMappings: uidMappings,
				GIDMappings: gidMappings,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeImageCloner.CreateCallCount()).To(Equal(1))
			_, createImagerSpec := fakeImageCloner.CreateArgsForCall(0)
			Expect(createImagerSpec).To(Equal(groot.ImageSpec{
				ID:            "some-id",
				BaseVolumeIDs: []string{"id-1", "id-2"},
				BaseImage: specsv1.Image{
					Author: "Groot",
				},
				OwnerUID: 50,
				OwnerGID: 60,
			}))
		})

		It("configures the rootfs", func() {
			injectedBaseImage := specsv1.Image{
				Config: specsv1.ImageConfig{
					Volumes: map[string]struct{}{
						"/path/to/volume": struct{}{},
					},
				},
			}
			fakeBaseImagePuller.PullReturns(groot.BaseImage{
				BaseImage: injectedBaseImage,
			}, nil)

			image, err := creator.Create(logger, groot.CreateSpec{
				ID:        "my-image",
				BaseImage: "docker:///ubuntu",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeRootFSConfigurer.ConfigureCallCount()).To(Equal(1))
			rootFSPath, baseImage := fakeRootFSConfigurer.ConfigureArgsForCall(0)
			Expect(rootFSPath).To(Equal(filepath.Join(image.Path, "rootfs")))
			Expect(baseImage).To(Equal(injectedBaseImage))
		})

		It("releases the global lock", func() {
			_, err := creator.Create(logger, groot.CreateSpec{
				BaseImage: "/path/to/image",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.UnlockArgsForCall(0)).To(Equal(lockFile))
		})

		It("returns the image", func() {
			expectedImage := groot.ImageInfo{
				Path:   "/path/to/image",
				Rootfs: "rootfs-path",
			}
			fakeImageCloner.CreateReturns(expectedImage, nil)

			image, err := creator.Create(logger, groot.CreateSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(image).To(Equal(expectedImage))
		})

		It("emits metrics for creation", func() {
			_, err := creator.Create(logger, groot.CreateSpec{
				ID:        "some-id",
				BaseImage: "/path/to/image",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeMetricsEmitter.TryEmitDurationFromCallCount()).To(Equal(1))
			_, name, start := fakeMetricsEmitter.TryEmitDurationFromArgsForCall(0)
			Expect(name).To(Equal(groot.MetricImageCreationTime))
			Expect(start).NotTo(BeZero())
		})

		Describe("store ownership", func() {
			BeforeEach(func() {
				baseImage := groot.BaseImage{
					BaseImage: specsv1.Image{
						Author: "Groot",
					},
				}
				fakeBaseImagePuller.PullReturns(baseImage, nil)
			})

			It("is defined by the root ID mappings", func() {
				uidMappings := []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 50, NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 51, NamespaceID: 1, Size: 100},
				}
				gidMappings := []groot.IDMappingSpec{
					groot.IDMappingSpec{HostID: 60, NamespaceID: 0, Size: 1},
					groot.IDMappingSpec{HostID: 61, NamespaceID: 1, Size: 300},
				}
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage:   "/path/to/image",
					UIDMappings: uidMappings,
					GIDMappings: gidMappings,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBaseImagePuller.PullCallCount()).To(Equal(1))
				_, imageSpec := fakeBaseImagePuller.PullArgsForCall(0)
				Expect(imageSpec.OwnerUID).To(Equal(50))
				Expect(imageSpec.OwnerGID).To(Equal(60))

				Expect(fakeImageCloner.CreateCallCount()).To(Equal(1))
				_, createImagerSpec := fakeImageCloner.CreateArgsForCall(0)
				Expect(createImagerSpec.OwnerUID).To(Equal(50))
				Expect(createImagerSpec.OwnerGID).To(Equal(60))
			})

			Context("when there's no root mapping", func() {
				It("sets the current user as the store owner", func() {
					_, err := creator.Create(logger, groot.CreateSpec{
						BaseImage: "/path/to/image",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBaseImagePuller.PullCallCount()).To(Equal(1))
					_, imageSpec := fakeBaseImagePuller.PullArgsForCall(0)
					Expect(imageSpec.OwnerUID).To(Equal(os.Getuid()))
					Expect(imageSpec.OwnerGID).To(Equal(os.Getgid()))

					Expect(fakeImageCloner.CreateCallCount()).To(Equal(1))
					_, createImagerSpec := fakeImageCloner.CreateArgsForCall(0)
					Expect(createImagerSpec.OwnerUID).To(Equal(os.Getuid()))
					Expect(createImagerSpec.OwnerGID).To(Equal(os.Getgid()))
				})
			})
		})

		Context("when the image is not a valid URL", func() {
			It("returns an error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "%%!!#@!^&",
				})
				Expect(err).To(MatchError(ContainSubstring("parsing image url")))
			})

			It("does not create a image", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "%%!!#@!^&",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImageCloner.CreateCallCount()).To(Equal(0))
			})
		})

		Context("when the id already exists", func() {
			BeforeEach(func() {
				fakeImageCloner.ExistsReturns(true, nil)
			})

			It("returns an error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
					ID:        "some-id",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImageCloner.CreateCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("image for id `some-id` already exists")))
			})

			It("does not pull the image", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
					ID:        "some-id",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBaseImagePuller.PullCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("image for id `some-id` already exists")))
			})
		})

		Context("when checking if the id exists fails", func() {
			BeforeEach(func() {
				fakeImageCloner.ExistsReturns(false, errors.New("Checking if the image ID exists"))
			})

			It("returns an error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImageCloner.CreateCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("Checking if the image ID exists")))
			})

			It("does not pull the image", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBaseImagePuller.PullCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("Checking if the image ID exists")))
			})
		})

		Context("when the id contains invalid characters", func() {
			It("returns an error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
					ID:        "some/id",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImageCloner.CreateCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("id `some/id` contains invalid characters: `/`")))
			})
		})

		Context("when acquiring the lock fails", func() {
			BeforeEach(func() {
				fakeLocksmith.LockReturns(nil, errors.New("failed to lock"))
			})

			It("returns the error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to lock")))
			})

			It("does not pull the image", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBaseImagePuller.PullCallCount()).To(BeZero())
			})
		})

		Context("when checking for namespace returns an error", func() {
			BeforeEach(func() {
				fakeNamespaceChecker.CheckReturns(false, errors.New("failed to check namespace"))
			})

			It("returns an error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to check namespace")))
			})
		})

		Context("when checking for namespace returns false", func() {
			BeforeEach(func() {
				fakeNamespaceChecker.CheckReturns(false, nil)
			})

			It("returns an error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(MatchError(ContainSubstring("store already initialized with a different mapping")))
			})
		})

		Context("when pulling the image fails", func() {
			BeforeEach(func() {
				fakeBaseImagePuller.PullReturns(groot.BaseImage{}, errors.New("failed to pull image"))
			})

			It("returns the error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to pull image")))
			})

			It("does not create a image", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					BaseImage: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImageCloner.CreateCallCount()).To(Equal(0))
			})
		})

		Context("when cloning the image fails", func() {
			BeforeEach(func() {
				fakeImageCloner.CreateReturns(groot.ImageInfo{}, errors.New("Failed to make image"))
			})

			It("returns the error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{})
				Expect(err).To(MatchError("making image: Failed to make image"))
			})
		})

		Context("when registering dependencies fails", func() {
			BeforeEach(func() {
				fakeDependencyManager.RegisterReturns(errors.New("failed to register dependencies"))
				fakeBaseImagePuller.PullReturns(groot.BaseImage{}, nil)
			})

			It("returns an errors", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					ID:        "my-image",
					BaseImage: "/path/to/image",
				})

				Expect(err).To(MatchError(ContainSubstring("failed to register dependencies")))
			})

			It("destroys the image", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					ID:        "my-image",
					BaseImage: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImageCloner.DestroyCallCount()).To(Equal(1))
			})

			It("does not configure the rootfs", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					ID:        "my-image",
					BaseImage: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeRootFSConfigurer.ConfigureCallCount()).To(Equal(0))
			})
		})

		Context("when configuring the rootfs fails", func() {
			BeforeEach(func() {
				fakeRootFSConfigurer.ConfigureReturns(errors.New("failed to configure rootfs"))
			})

			It("returns an error", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					ID:        "my-image",
					BaseImage: "docker:///ubuntu",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to configure rootfs")))
			})

			It("destroys the image", func() {
				_, err := creator.Create(logger, groot.CreateSpec{
					ID:        "my-image",
					BaseImage: "docker:///ubuntu",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImageCloner.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when disk limit is given", func() {
			It("passes the disk limit to the imageCloner", func() {
				baseImage := groot.BaseImage{
					ChainIDs: []string{"id-1", "id-2"},
					BaseImage: specsv1.Image{
						Author: "Groot",
					},
				}
				fakeBaseImagePuller.PullReturns(baseImage, nil)

				_, err := creator.Create(logger, groot.CreateSpec{
					ID:        "some-id",
					DiskLimit: int64(1024),
					BaseImage: "/path/to/image",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeImageCloner.CreateCallCount()).To(Equal(1))
				_, createImagerSpec := fakeImageCloner.CreateArgsForCall(0)
				Expect(createImagerSpec).To(Equal(groot.ImageSpec{
					ID:            "some-id",
					BaseVolumeIDs: []string{"id-1", "id-2"},
					BaseImage: specsv1.Image{
						Author: "Groot",
					},
					OwnerUID:  os.Getuid(),
					OwnerGID:  os.Getgid(),
					DiskLimit: int64(1024),
				}))
			})
		})
	})
})

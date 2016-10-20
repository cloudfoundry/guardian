package groot_test

import (
	"errors"
	"io/ioutil"
	"net/url"
	"os"

	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("I AM GROOT, the Orchestrator", func() {
	var (
		fakeBundler           *grootfakes.FakeBundler
		fakeImagePuller       *grootfakes.FakeImagePuller
		fakeLocksmith         *grootfakes.FakeLocksmith
		fakeDependencyManager *grootfakes.FakeDependencyManager
		fakeGarbageCollector  *grootfakes.FakeGarbageCollector
		lockFile              *os.File

		groot  *grootpkg.Groot
		logger lager.Logger
	)

	BeforeEach(func() {
		var err error

		fakeBundler = new(grootfakes.FakeBundler)
		fakeImagePuller = new(grootfakes.FakeImagePuller)

		fakeLocksmith = new(grootfakes.FakeLocksmith)
		lockFile, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		fakeLocksmith.LockReturns(lockFile, nil)
		fakeDependencyManager = new(grootfakes.FakeDependencyManager)
		fakeGarbageCollector = new(grootfakes.FakeGarbageCollector)

		groot = grootpkg.IamGroot(fakeBundler, fakeImagePuller, fakeLocksmith, fakeDependencyManager, fakeGarbageCollector)
		logger = lagertest.NewTestLogger("groot")
	})

	AfterEach(func() {
		Expect(os.Remove(lockFile.Name())).To(Succeed())
	})

	Describe("Create", func() {
		It("acquires the global lock", func() {
			_, err := groot.Create(logger, grootpkg.CreateSpec{
				Image: "/path/to/image",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.LockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.LockArgsForCall(0)).To(Equal(grootpkg.GLOBAL_LOCK_KEY))
		})

		It("pulls the image", func() {
			uidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 1, NamespaceID: 2, Size: 10}}
			gidMappings := []grootpkg.IDMappingSpec{grootpkg.IDMappingSpec{HostID: 10, NamespaceID: 20, Size: 100}}

			_, err := groot.Create(logger, grootpkg.CreateSpec{
				Image:       "/path/to/image",
				UIDMappings: uidMappings,
				GIDMappings: gidMappings,
			})
			Expect(err).NotTo(HaveOccurred())

			imageURL, err := url.Parse("/path/to/image")
			Expect(err).NotTo(HaveOccurred())
			_, imageSpec := fakeImagePuller.PullArgsForCall(0)
			Expect(imageSpec.ImageSrc).To(Equal(imageURL))
			Expect(imageSpec.UIDMappings).To(Equal(uidMappings))
			Expect(imageSpec.GIDMappings).To(Equal(gidMappings))
		})

		It("releases the global lock", func() {
			_, err := groot.Create(logger, grootpkg.CreateSpec{
				Image: "/path/to/image",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.UnlockArgsForCall(0)).To(Equal(lockFile))
		})

		It("makes a bundle", func() {
			image := grootpkg.Image{
				VolumePath: "/path/to/volume",
				Image: specsv1.Image{
					Author: "Groot",
				},
			}
			fakeImagePuller.PullReturns(image, nil)

			_, err := groot.Create(logger, grootpkg.CreateSpec{
				ID:    "some-id",
				Image: "/path/to/image",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBundler.CreateCallCount()).To(Equal(1))
			_, createBundlerSpec := fakeBundler.CreateArgsForCall(0)
			Expect(createBundlerSpec).To(Equal(grootpkg.BundleSpec{
				ID:         "some-id",
				VolumePath: "/path/to/volume",
				Image: specsv1.Image{
					Author: "Groot",
				},
			}))
		})

		It("registers chain ids used by a bundle", func() {
			image := grootpkg.Image{
				ChainIDs: []string{"sha256:vol-a", "sha256:vol-b"},
			}
			fakeImagePuller.PullReturns(image, nil)

			_, err := groot.Create(logger, grootpkg.CreateSpec{
				ID:    "my-bundle",
				Image: "/path/to/image",
			})

			Expect(err).NotTo(HaveOccurred())

			bundleID, chainIDs := fakeDependencyManager.RegisterArgsForCall(0)
			Expect(bundleID).To(Equal("my-bundle"))
			Expect(chainIDs).To(Equal([]string{"sha256:vol-a", "sha256:vol-b"}))
		})

		It("returns the bundle", func() {
			fakeBundler.CreateReturns(grootpkg.Bundle{
				Path: "/path/to/bundle",
			}, nil)

			bundle, err := groot.Create(logger, grootpkg.CreateSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.Path).To(Equal("/path/to/bundle"))
		})

		Context("when the image is not a valid URL", func() {
			It("returns an error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "%%!!#@!^&",
				})
				Expect(err).To(MatchError(ContainSubstring("parsing image url")))
			})

			It("does not create a bundle", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "%%!!#@!^&",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
			})
		})

		Context("when the id already exists", func() {
			BeforeEach(func() {
				fakeBundler.ExistsReturns(true, nil)
			})

			It("returns an error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
					ID:    "some-id",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("bundle for id `some-id` already exists")))
			})

			It("does not pull the image", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
					ID:    "some-id",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImagePuller.PullCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("bundle for id `some-id` already exists")))
			})
		})

		Context("when checking if the id exists fails", func() {
			BeforeEach(func() {
				fakeBundler.ExistsReturns(false, errors.New("Checking if the bundle ID exists"))
			})

			It("returns an error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("Checking if the bundle ID exists")))
			})

			It("does not pull the image", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImagePuller.PullCallCount()).To(Equal(0))
				Expect(err).To(MatchError(ContainSubstring("Checking if the bundle ID exists")))
			})
		})

		Context("when acquiring the lock fails", func() {
			BeforeEach(func() {
				fakeLocksmith.LockReturns(nil, errors.New("failed to lock"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to lock")))
			})

			It("does not pull the image", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeImagePuller.PullCallCount()).To(BeZero())
			})
		})

		Context("when pulling the image fails", func() {
			BeforeEach(func() {
				fakeImagePuller.PullReturns(grootpkg.Image{}, errors.New("failed to pull image"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(MatchError(ContainSubstring("failed to pull image")))
			})

			It("does not create a bundle", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(0))
			})

			It("releases the global lock", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					Image: "/path/to/image",
				})
				Expect(err).To(HaveOccurred())

				Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
				Expect(fakeLocksmith.UnlockArgsForCall(0)).To(Equal(lockFile))
			})
		})

		Context("when creating the bundle fails", func() {
			BeforeEach(func() {
				fakeBundler.CreateReturns(grootpkg.Bundle{}, errors.New("Failed to make bundle"))
			})

			It("returns the error", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{})
				Expect(err).To(MatchError("making bundle: Failed to make bundle"))
			})
		})

		Context("when registering dependencies fails", func() {
			BeforeEach(func() {
				fakeDependencyManager.RegisterReturns(errors.New("failed to register dependencies"))
				fakeImagePuller.PullReturns(grootpkg.Image{}, nil)
			})

			It("returns an errors", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					ID:    "my-bundle",
					Image: "/path/to/image",
				})

				Expect(err).To(MatchError(ContainSubstring("failed to register dependencies")))
			})

			It("destroys the bundle", func() {
				_, err := groot.Create(logger, grootpkg.CreateSpec{
					ID:    "my-bundle",
					Image: "/path/to/image",
				})

				Expect(err).To(HaveOccurred())
				Expect(fakeBundler.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when disk limit is given", func() {
			It("applies the disk limit", func() {
				image := grootpkg.Image{
					VolumePath: "/path/to/volume",
					Image: specsv1.Image{
						Author: "Groot",
					},
				}
				fakeImagePuller.PullReturns(image, nil)

				_, err := groot.Create(logger, grootpkg.CreateSpec{
					ID:        "some-id",
					DiskLimit: int64(1024),
					Image:     "/path/to/image",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBundler.CreateCallCount()).To(Equal(1))
				_, createBundlerSpec := fakeBundler.CreateArgsForCall(0)
				Expect(createBundlerSpec).To(Equal(grootpkg.BundleSpec{
					ID:         "some-id",
					VolumePath: "/path/to/volume",
					Image: specsv1.Image{
						Author: "Groot",
					},
					DiskLimit: int64(1024),
				}))
			})
		})
	})

	Describe("Delete", func() {
		It("destroys a bundle", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())

			_, bundleId := fakeBundler.DestroyArgsForCall(0)
			Expect(bundleId).To(Equal("some-id"))
		})

		It("deregisters bundle dependencies", func() {
			Expect(groot.Delete(logger, "some-id")).To(Succeed())
			Expect(fakeDependencyManager.DeregisterCallCount()).To(Equal(1))
		})

		Context("when destroying a bundle fails", func() {
			BeforeEach(func() {
				fakeBundler.DestroyReturns(errors.New("failed to destroy bundle"))
			})

			It("returns an error", func() {
				Expect(groot.Delete(logger, "some-id")).To(MatchError(ContainSubstring("failed to destroy bundle")))
			})
		})
	})

	Describe("Metrics", func() {
		It("asks for metrics from the bundler", func() {
			fakeBundler.MetricsReturns(grootpkg.VolumeMetrics{}, nil)
			_, err := groot.Metrics(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeBundler.MetricsCallCount()).To(Equal(1))
			_, id := fakeBundler.MetricsArgsForCall(0)
			Expect(id).To(Equal("some-id"))
		})

		It("asks for metrics from the bundler", func() {
			metrics := grootpkg.VolumeMetrics{
				DiskUsage: grootpkg.DiskUsage{
					TotalBytesUsed:     1024,
					ExclusiveBytesUsed: 512,
				},
			}
			fakeBundler.MetricsReturns(metrics, nil)

			returnedMetrics, err := groot.Metrics(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedMetrics).To(Equal(metrics))
		})

		Context("when bundler fails", func() {
			It("returns an error", func() {
				fakeBundler.MetricsReturns(grootpkg.VolumeMetrics{}, errors.New("sorry"))

				_, err := groot.Metrics(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("sorry")))
			})
		})
	})

	Describe("Clean", func() {
		It("collects orphaned volumes", func() {
			Expect(groot.Clean(logger)).To(Succeed())
			Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(1))
		})

		Context("when garbage collecting fails", func() {
			BeforeEach(func() {
				fakeGarbageCollector.CollectReturns(errors.New("failed to collect unused bits"))
			})

			It("returns an error", func() {
				Expect(groot.Clean(logger)).To(MatchError(ContainSubstring("failed to collect unused bits")))
			})
		})

		It("acquires the global lock", func() {
			Expect(groot.Clean(logger)).To(Succeed())

			Expect(fakeLocksmith.LockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.LockArgsForCall(0)).To(Equal(grootpkg.GLOBAL_LOCK_KEY))
		})

		Context("when acquiring the lock fails", func() {
			BeforeEach(func() {
				fakeLocksmith.LockReturns(nil, errors.New("failed to acquire lock"))
			})

			It("returns the error", func() {
				err := groot.Clean(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to acquire lock")))
			})

			It("does not collect the garbage", func() {
				Expect(groot.Clean(logger)).NotTo(Succeed())
				Expect(fakeGarbageCollector.CollectCallCount()).To(Equal(0))
			})
		})

		It("releases the global lock", func() {
			Expect(groot.Clean(logger)).To(Succeed())

			Expect(fakeLocksmith.UnlockCallCount()).To(Equal(1))
			Expect(fakeLocksmith.UnlockArgsForCall(0)).To(Equal(lockFile))
		})
	})
})

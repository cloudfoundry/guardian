package bundler_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	bundlerpkg "code.cloudfoundry.org/grootfs/store/bundler"
	"code.cloudfoundry.org/grootfs/store/bundler/bundlerfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bundle", func() {
	var (
		logger      lager.Logger
		storePath   string
		bundlesPath string
		bundler     *bundlerpkg.Bundler

		fakeSnapshotDriver *bundlerfakes.FakeSnapshotDriver
	)

	BeforeEach(func() {
		var err error
		fakeSnapshotDriver = new(bundlerfakes.FakeSnapshotDriver)

		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		bundlesPath = filepath.Join(storePath, "bundles")

		Expect(os.Mkdir(bundlesPath, 0777)).To(Succeed())
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-bunlder")
		bundler = bundlerpkg.NewBundler(fakeSnapshotDriver, storePath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("BundleIDs", func() {
		createBundle := func(name string, layers []string) {
			bundlePath := filepath.Join(bundlesPath, name)
			Expect(os.Mkdir(bundlePath, 0777)).To(Succeed())
			l := struct {
				Layers []string `json:"layers"`
			}{
				Layers: layers,
			}
			bundleJson, err := json.Marshal(l)
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "bundle.json"), bundleJson, 0644)).To(Succeed())
		}

		BeforeEach(func() {
			createBundle("bundle-a", []string{"sha-1", "sha-2"})
			createBundle("bundle-b", []string{"sha-1", "sha-3", "sha-4"})
		})

		It("returns a list with all known bundles", func() {
			bundles, err := bundler.BundleIDs(logger)
			Expect(err).NotTo(HaveOccurred())
			Expect(bundles).To(ConsistOf("bundle-a", "bundle-b"))
		})

		Context("when fails to list bundles", func() {
			BeforeEach(func() {
				Expect(os.Chmod(bundlesPath, 0666)).To(Succeed())
			})

			AfterEach(func() {
				// we need to revert permissions because of the outer AfterEach
				Expect(os.Chmod(bundlesPath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := bundler.BundleIDs(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to read bundles dir")))
			})
		})
	})

	Describe("Create", func() {
		It("returns a bundle directory", func() {
			bundle, err := bundler.Create(logger, groot.BundleSpec{ID: "some-id"})
			Expect(err).NotTo(HaveOccurred())
			Expect(bundle.Path()).To(BeADirectory())
		})

		It("keeps the bundles in the same bundle directory", func() {
			someBundle, err := bundler.Create(logger, groot.BundleSpec{ID: "some-id"})
			Expect(err).NotTo(HaveOccurred())
			anotherBundle, err := bundler.Create(logger, groot.BundleSpec{ID: "another-id"})
			Expect(err).NotTo(HaveOccurred())

			Expect(someBundle.Path()).NotTo(BeEmpty())
			Expect(anotherBundle.Path()).NotTo(BeEmpty())

			bundles, err := ioutil.ReadDir(path.Join(storePath, store.BUNDLES_DIR_NAME))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(bundles)).To(Equal(2))
		})

		Context("when calling it with two different ids", func() {
			It("returns two different bundle paths", func() {
				bundle, err := bundler.Create(logger, groot.BundleSpec{ID: "some-id"})
				Expect(err).NotTo(HaveOccurred())

				anotherBundle, err := bundler.Create(logger, groot.BundleSpec{ID: "another-id"})
				Expect(err).NotTo(HaveOccurred())

				Expect(bundle.Path()).NotTo(Equal(anotherBundle.Path()))
			})
		})

		Context("when the store path does not exist", func() {
			BeforeEach(func() {
				storePath = "/non/existing/store"
			})

			It("should return an error", func() {
				_, err := bundler.Create(logger, groot.BundleSpec{ID: "some-id"})
				Expect(err).To(MatchError(ContainSubstring("making bundle path")))
			})
		})

		It("creates the snapshot", func() {
			bundleSpec := groot.BundleSpec{
				ID:         "some-id",
				VolumePath: "/path/to/volume",
				Image: specsv1.Image{
					Author: "Groot",
				},
			}
			bundle, err := bundler.Create(logger, bundleSpec)
			Expect(err).NotTo(HaveOccurred())

			_, fromPath, toPath := fakeSnapshotDriver.SnapshotArgsForCall(0)
			Expect(fromPath).To(Equal(bundleSpec.VolumePath))
			Expect(toPath).To(Equal(bundle.RootFSPath()))
		})

		Context("when creating the snapshot fails", func() {
			BeforeEach(func() {
				fakeSnapshotDriver.SnapshotReturns(errors.New("failed to create snapshot"))
			})

			It("returns an error", func() {
				_, err := bundler.Create(logger, groot.BundleSpec{ID: "some-id"})
				Expect(err).To(MatchError(ContainSubstring("failed to create snapshot")))
			})

			It("removes the bundle", func() {
				bundleID := "some-id"
				_, err := bundler.Create(logger, groot.BundleSpec{ID: bundleID})
				Expect(err).To(HaveOccurred())
				Expect(filepath.Join(bundlesPath, bundleID)).NotTo(BeADirectory())
			})
		})

		It("writes the image.json to the bundle", func() {
			image := specsv1.Image{
				Author: "Groot",
				Config: specsv1.ImageConfig{
					User: "groot",
				},
			}

			bundle, err := bundler.Create(logger, groot.BundleSpec{
				ID:         "some-id",
				VolumePath: "/path/to/volume",
				Image:      image,
			})
			Expect(err).NotTo(HaveOccurred())

			imageJsonPath := filepath.Join(bundle.Path(), "image.json")
			Expect(imageJsonPath).To(BeAnExistingFile())

			imageJsonFile, err := os.Open(imageJsonPath)
			Expect(err).NotTo(HaveOccurred())

			var imageJsonContent specsv1.Image
			Expect(json.NewDecoder(imageJsonFile).Decode(&imageJsonContent)).To(Succeed())
			Expect(imageJsonContent).To(Equal(image))
		})

		Context("when writting the image.json fails", func() {
			BeforeEach(func() {
				bundlerpkg.OF = func(name string, flag int, perm os.FileMode) (*os.File, error) {
					return nil, errors.New("permission denied: can't write stuff")
				}
			})

			AfterEach(func() {
				// needs to reasign the correct method after running the test
				bundlerpkg.OF = os.OpenFile
			})

			It("returns an error", func() {
				_, err := bundler.Create(logger, groot.BundleSpec{ID: "some-id"})
				Expect(err).To(MatchError(ContainSubstring("permission denied: can't write stuff")))
			})

			It("removes the bundle", func() {
				bundleID := "some-id"
				_, err := bundler.Create(logger, groot.BundleSpec{ID: bundleID})
				Expect(err).To(HaveOccurred())
				Expect(filepath.Join(bundlesPath, bundleID)).NotTo(BeADirectory())
			})
		})

		Context("when a disk limit is set", func() {
			It("applies the disk limit", func() {
				bundle, err := bundler.Create(logger, groot.BundleSpec{
					ID:        "some-id",
					DiskLimit: int64(1024),
				})
				Expect(err).NotTo(HaveOccurred())

				_, path, diskLimit, excludeImageFromQuota := fakeSnapshotDriver.ApplyDiskLimitArgsForCall(0)
				Expect(path).To(Equal(bundle.RootFSPath()))
				Expect(diskLimit).To(Equal(int64(1024)))
				Expect(excludeImageFromQuota).To(BeFalse())
			})

			Context("when applying the disk limit fails", func() {
				BeforeEach(func() {
					fakeSnapshotDriver.ApplyDiskLimitReturns(errors.New("failed to apply disk limit"))
				})

				It("returns an error", func() {
					_, err := bundler.Create(logger, groot.BundleSpec{
						ID:        "some-id",
						DiskLimit: int64(1024),
					})

					Expect(err).To(MatchError(ContainSubstring("failed to apply disk limit")))
				})

				It("removes the snapshot", func() {
					_, err := bundler.Create(logger, groot.BundleSpec{
						ID:        "some-id",
						DiskLimit: int64(1024),
					})
					Expect(err).To(HaveOccurred())
					Expect(fakeSnapshotDriver.DestroyCallCount()).To(Equal(1))
					_, bundlePath := fakeSnapshotDriver.DestroyArgsForCall(0)
					Expect(bundlePath).To(Equal(bundlePath))
				})
			})

			Context("when the exclusive flag is set", func() {
				It("enforces the exclusive limit", func() {
					_, err := bundler.Create(logger, groot.BundleSpec{
						ID:                    "some-id",
						DiskLimit:             int64(1024),
						ExcludeImageFromQuota: true,
					})
					Expect(err).NotTo(HaveOccurred())
					_, _, _, excludeImageFromQuota := fakeSnapshotDriver.ApplyDiskLimitArgsForCall(0)
					Expect(excludeImageFromQuota).To(BeTrue())
				})
			})
		})
	})

	Describe("Destroy", func() {
		var bundlePath, bundleRootFSPath string

		BeforeEach(func() {
			bundlePath = path.Join(storePath, store.BUNDLES_DIR_NAME, "some-id")
			bundleRootFSPath = path.Join(bundlePath, "rootfs")
			Expect(os.MkdirAll(bundlePath, 0755)).To(Succeed())
			Expect(os.MkdirAll(bundleRootFSPath, 0755)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(bundlePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		})

		It("deletes an existing bundle", func() {
			Expect(bundler.Destroy(logger, "some-id")).To(Succeed())
			Expect(bundlePath).NotTo(BeAnExistingFile())
		})

		Context("when bundle does not exist", func() {
			It("returns an error", func() {
				err := bundler.Destroy(logger, "cake")
				Expect(err).To(MatchError(ContainSubstring("bundle not found")))
			})
		})

		Context("when deleting the folder fails", func() {
			BeforeEach(func() {
				Expect(os.Chmod(bundlePath, 0666)).To(Succeed())
			})

			AfterEach(func() {
				// we need to revert permissions because of the outer AfterEach
				Expect(os.Chmod(bundlePath, 0755)).To(Succeed())
			})

			It("returns an error", func() {
				err := bundler.Destroy(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("deleting bundle path")))
			})
		})

		It("removes the snapshot", func() {
			err := bundler.Destroy(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())

			_, path := fakeSnapshotDriver.DestroyArgsForCall(0)
			Expect(path).To(Equal(bundleRootFSPath))
		})

		Context("when removing the snapshot fails", func() {
			BeforeEach(func() {
				fakeSnapshotDriver.DestroyReturns(errors.New("failed to remove snapshot"))
			})

			It("returns an error", func() {
				err := bundler.Destroy(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("failed to remove snapshot")))
			})
		})
	})

	Describe("Exists", func() {
		BeforeEach(func() {
			Expect(os.Mkdir(filepath.Join(bundlesPath, "some-id"), 0777)).To(Succeed())
		})

		It("returns true when bundle exists", func() {
			ok, err := bundler.Exists("some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
		})

		Context("when bundle does not exist", func() {
			It("returns false", func() {
				ok, err := bundler.Exists("invalid-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeFalse())
			})

			Context("when it does not have permission to check", func() {
				JustBeforeEach(func() {
					basePath, err := ioutil.TempDir("", "")
					Expect(err).NotTo(HaveOccurred())

					noPermStorePath := filepath.Join(basePath, "no-perm-dir")
					Expect(os.Mkdir(noPermStorePath, 0000)).To(Succeed())

					bundler = bundlerpkg.NewBundler(fakeSnapshotDriver, noPermStorePath)
				})

				It("returns an error", func() {
					ok, err := bundler.Exists("invalid-id")
					Expect(err).To(MatchError(ContainSubstring("stat")))
					Expect(ok).To(BeFalse())
				})
			})
		})
	})
	Describe("Metrics", func() {
		var (
			bundlePath, bundleRootFSPath string
			metrics                      groot.VolumeMetrics
		)

		BeforeEach(func() {
			metrics = groot.VolumeMetrics{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     int64(1024),
					ExclusiveBytesUsed: int64(1024),
				},
			}

			bundlePath = path.Join(storePath, store.BUNDLES_DIR_NAME, "some-id")
			bundleRootFSPath = path.Join(bundlePath, "rootfs")
		})

		It("fetches the metrics", func() {
			_, err := bundler.Metrics(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeSnapshotDriver.FetchMetricsCallCount()).To(Equal(1))
		})

		It("returns the metrics", func() {
			fakeSnapshotDriver.FetchMetricsReturns(metrics, nil)

			m, err := bundler.Metrics(logger, "some-id")

			Expect(err).ToNot(HaveOccurred())
			Expect(m).To(Equal(metrics))
		})

		Context("when the snapshot driver fails", func() {
			It("returns an error", func() {
				fakeSnapshotDriver.FetchMetricsReturns(groot.VolumeMetrics{}, errors.New("failed"))

				_, err := bundler.Metrics(logger, "some-id")
				Expect(err).To(MatchError("failed"))
			})
		})
	})
})

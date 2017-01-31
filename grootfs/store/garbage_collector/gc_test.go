package garbage_collector_test

import (
	"errors"

	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/garbage_collector/garbage_collectorfakes"
	"code.cloudfoundry.org/grootfs/store/storefakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gc", func() {

	var (
		logger                lager.Logger
		garbageCollector      *garbage_collector.GarbageCollector
		fakeCacheDriver       *garbage_collectorfakes.FakeCacheDriver
		fakeVolumeDriver      *storefakes.FakeVolumeDriver
		fakeDependencyManager *garbage_collectorfakes.FakeDependencyManager
		fakeImageCloner       *garbage_collectorfakes.FakeImageCloner
	)

	BeforeEach(func() {
		fakeImageCloner = new(garbage_collectorfakes.FakeImageCloner)
		fakeCacheDriver = new(garbage_collectorfakes.FakeCacheDriver)
		fakeVolumeDriver = new(storefakes.FakeVolumeDriver)
		fakeDependencyManager = new(garbage_collectorfakes.FakeDependencyManager)

		garbageCollector = garbage_collector.NewGC(fakeCacheDriver, fakeVolumeDriver, fakeImageCloner, fakeDependencyManager)

		logger = lagertest.NewTestLogger("garbage_collector")
	})

	Describe("Collect", func() {
		BeforeEach(func() {
			fakeVolumeDriver.VolumesReturns([]string{
				"sha256:vol-a",
				"sha256:vol-b",
				"sha256:vol-c",
				"sha256:vol-d",
				"sha256:vol-e",
			}, nil)

			fakeDependencyManager.DependenciesStub = func(id string) ([]string, error) {
				return map[string][]string{
					"image:idA":                         []string{"sha256:vol-a", "sha256:vol-b"},
					"image:idB":                         []string{"sha256:vol-a", "sha256:vol-c"},
					"baseimage:docker:///ubuntu":        []string{"sha256:vol-d"},
					"baseimage:docker://private/ubuntu": []string{"sha256:vol-e"},
				}[id], nil
			}

			fakeImageCloner.ImageIDsReturns([]string{"idA", "idB"}, nil)
		})

		It("collects unused volumes", func() {
			Expect(garbageCollector.Collect(logger, []string{})).To(Succeed())

			Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(2))
			volumes := []string{}
			_, volID := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
			volumes = append(volumes, volID)
			_, volID = fakeVolumeDriver.DestroyVolumeArgsForCall(1)
			volumes = append(volumes, volID)

			Expect(volumes).To(ContainElement("sha256:vol-d"))
			Expect(volumes).To(ContainElement("sha256:vol-e"))
		})

		It("collects blobs from the cache", func() {
			Expect(garbageCollector.Collect(logger, []string{})).To(Succeed())
			Expect(fakeCacheDriver.CleanCallCount()).To(Equal(1))
		})

		Context("when a list of images to keep is provided", func() {
			It("does not collect the unused volumes for those listed", func() {
				Expect(garbageCollector.Collect(logger, []string{"docker:///ubuntu"})).To(Succeed())

				Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(1))
				_, volID := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
				Expect(volID).To(Equal("sha256:vol-e"))
			})

			Context("when the image to keep is from a private registry", func() {
				It("does not collect the unused volumes for those listed", func() {
					Expect(garbageCollector.Collect(logger, []string{"docker://private/ubuntu"})).To(Succeed())

					Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(1))
					_, volID := fakeVolumeDriver.DestroyVolumeArgsForCall(0)
					Expect(volID).To(Equal("sha256:vol-d"))
				})
			})
		})

		Context("when destroying a volume fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.DestroyVolumeStub = func(_ lager.Logger, volID string) error {
					if volID == "sha256:vol-d" {
						return errors.New("failed to destroy volume")
					}

					return nil
				}
			})

			It("does not stop cleaning up remainging volumes", func() {
				Expect(garbageCollector.Collect(logger, []string{})).To(MatchError(ContainSubstring("destroying volumes failed")))
				Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(2))
			})
		})

		Context("when retrieving volume list fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumesReturns(nil, errors.New("failed to retrieve volume list"))
			})

			It("returns an error", func() {
				Expect(garbageCollector.Collect(logger, []string{})).To(MatchError(ContainSubstring("failed to retrieve volume list")))
			})
		})

		Context("when retrieving images fails", func() {
			BeforeEach(func() {
				fakeImageCloner.ImageIDsReturns(nil, errors.New("failed to retrieve images"))
			})

			It("returns an error", func() {
				Expect(garbageCollector.Collect(logger, []string{})).To(MatchError(ContainSubstring("failed to retrieve images")))
			})
		})

		Context("when getting the dependencies of a image fails", func() {
			BeforeEach(func() {
				fakeDependencyManager.DependenciesReturns(nil, errors.New("failed to access deps"))
			})

			It("returns an error", func() {
				Expect(garbageCollector.Collect(logger, []string{})).To(MatchError(
					ContainSubstring("failed to access deps"),
				))
			})

			It("does not delete any volumes", func() {
				Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(0))
			})
		})

		Context("when cleaning blobs cache fails", func() {
			BeforeEach(func() {
				fakeCacheDriver.CleanReturns(errors.New("failed to clean up cache"))
			})

			It("returns an error", func() {
				Expect(garbageCollector.Collect(logger, []string{})).To(MatchError(ContainSubstring("failed to clean up cache")))
			})
		})
	})
})

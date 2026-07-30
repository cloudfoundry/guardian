package garbage_collector_test

import (
	"errors"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/garbage_collector/garbage_collectorfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gc", func() {

	var (
		logger                lager.Logger
		garbageCollector      *garbage_collector.GarbageCollector
		fakeVolumeDriver      *garbage_collectorfakes.FakeVolumeDriver
		fakeDependencyManager *garbage_collectorfakes.FakeDependencyManager
		fakeImageIDsGetter    *garbage_collectorfakes.FakeImageIDsGetter
	)

	BeforeEach(func() {
		fakeImageIDsGetter = new(garbage_collectorfakes.FakeImageIDsGetter)
		fakeVolumeDriver = new(garbage_collectorfakes.FakeVolumeDriver)
		fakeDependencyManager = new(garbage_collectorfakes.FakeDependencyManager)

		logger = lagertest.NewTestLogger("garbage_collector")
	})

	JustBeforeEach(func() {
		garbageCollector = garbage_collector.NewGC(fakeVolumeDriver, fakeImageIDsGetter, fakeDependencyManager)
	})

	Describe("UnusedVolumes", func() {
		BeforeEach(func() {
			fakeVolumeDriver.VolumePathStub = func(_ lager.Logger, id string) (string, error) {
				return filepath.Join("/store/volumes", id), nil
			}

			fakeVolumeDriver.VolumesReturns([]string{
				"volDocker1",
				"volDocker2",
				"volDocker3",
				"usedLocalVolume-timestamp",
				"unusedLayerVolume",
				"unusedLocalVolume-timestamp",
				"sha256ubuntu",
				"sha256privateubuntu",
				"gc.markedUnusedVolume",
			}, nil)

			fakeDependencyManager.DependenciesStub = func(id string) ([]string, error) {
				return map[string][]string{
					"image:idA":                         []string{"volDocker1", "volDocker2"},
					"image:idB":                         []string{"volDocker1", "volDocker3"},
					"image:idLocal":                     []string{"usedLocalVolume-timestamp"},
					"baseimage:docker:///ubuntu":        []string{"sha256ubuntu"},
					"baseimage:docker://private/ubuntu": []string{"sha256privateubuntu"},
				}[id], nil
			}

			fakeImageIDsGetter.ImageIDsReturns([]string{"idA", "idB", "idLocal"}, nil)
		})

		It("retrieves the names of unused volumes", func() {
			unusedVolumes, err := garbageCollector.UnusedVolumes(logger)
			Expect(err).NotTo(HaveOccurred())

			Expect(unusedVolumes).To(ConsistOf("sha256ubuntu", "sha256privateubuntu", "unusedLayerVolume", "unusedLocalVolume-timestamp"))
		})

		Context("when retrieving images fails", func() {
			BeforeEach(func() {
				fakeImageIDsGetter.ImageIDsReturns(nil, errors.New("failed to retrieve images"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to retrieve images")))
			})
		})

		Context("when getting the dependencies of a image fails", func() {
			BeforeEach(func() {
				fakeDependencyManager.DependenciesReturns(nil, errors.New("failed to access deps"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to access deps")))
			})
		})

		Context("when retrieving volume list fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumesReturns(nil, errors.New("failed to retrieve volume list"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger)
				Expect(err).To(MatchError(ContainSubstring("failed to retrieve volume list")))
			})
		})
	})

	Describe("MarkUnused", func() {
		var (
			unusedVolumes []string
		)

		BeforeEach(func() {
			unusedVolumes = []string{"vol-d", "vol-e"}
		})

		It("marks unused volume artifacts", func() {
			Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(Succeed())
			Expect(fakeVolumeDriver.MarkVolumeArtifactsCallCount()).To(Equal(2))
			_, volID := fakeVolumeDriver.MarkVolumeArtifactsArgsForCall(0)
			Expect(volID).To(Equal("vol-d"))

			_, volID2 := fakeVolumeDriver.MarkVolumeArtifactsArgsForCall(1)
			Expect(volID2).To(Equal("vol-e"))
		})

		It("doesn't remark volume artifacts for gc", func() {
			Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(Succeed())

			for i := 0; i < fakeVolumeDriver.MarkVolumeArtifactsCallCount(); i++ {
				_, volID := fakeVolumeDriver.MarkVolumeArtifactsArgsForCall(i)
				Expect(volID).NotTo(Equal("gc.vol-f"))
			}
		})

		Context("when marking unused volume artifacts fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.MarkVolumeArtifactsReturns(errors.New("Failed to move"))
			})

			It("returns an error", func() {
				Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(MatchError(ContainSubstring("2/2 volumes failed to be marked as unused")))
			})
		})
	})

	Describe("Collect", func() {
		BeforeEach(func() {
			fakeVolumeDriver.VolumesReturns([]string{
				"vol-a",
				"gc.vol-b",
				"gc.vol-c",
				"vol-d",
				"vol-e",
				"gc.vol-f",
			}, nil)
		})

		It("collects unused volumes", func() {
			Expect(garbageCollector.Collect(logger)).To(Succeed())

			Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(3))
			volumes := []string{}
			for i := 0; i < fakeVolumeDriver.DestroyVolumeCallCount(); i++ {
				_, volID := fakeVolumeDriver.DestroyVolumeArgsForCall(i)
				volumes = append(volumes, volID)
			}

			Expect(volumes).To(ContainElement("gc.vol-b"))
			Expect(volumes).To(ContainElement("gc.vol-c"))
			Expect(volumes).To(ContainElement("gc.vol-f"))
		})

		Context("when destroying a volume fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.DestroyVolumeStub = func(_ lager.Logger, volID string) error {
					if volID == "gc.vol-f" {
						return errors.New("failed to destroy volume")
					}

					return nil
				}
			})

			It("does not stop cleaning up remaining volumes", func() {
				Expect(garbageCollector.Collect(logger)).To(MatchError(ContainSubstring("destroying volumes failed")))
				Expect(fakeVolumeDriver.DestroyVolumeCallCount()).To(Equal(3))
			})
		})
	})
})

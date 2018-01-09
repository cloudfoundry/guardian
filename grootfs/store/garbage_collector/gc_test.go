package garbage_collector_test

import (
	"errors"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store/garbage_collector"
	"code.cloudfoundry.org/grootfs/store/garbage_collector/garbage_collectorfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gc", func() {

	var (
		logger                lager.Logger
		garbageCollector      *garbage_collector.GarbageCollector
		fakeVolumeDriver      *garbage_collectorfakes.FakeVolumeDriver
		fakeDependencyManager *garbage_collectorfakes.FakeDependencyManager
		fakeImageCloner       *garbage_collectorfakes.FakeImageCloner
	)

	BeforeEach(func() {
		fakeImageCloner = new(garbage_collectorfakes.FakeImageCloner)
		fakeVolumeDriver = new(garbage_collectorfakes.FakeVolumeDriver)
		fakeDependencyManager = new(garbage_collectorfakes.FakeDependencyManager)

		logger = lagertest.NewTestLogger("garbage_collector")
	})

	JustBeforeEach(func() {
		garbageCollector = garbage_collector.NewGC(fakeVolumeDriver, fakeImageCloner, fakeDependencyManager)
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

			fakeImageCloner.ImageIDsReturns([]string{"idA", "idB", "idLocal"}, nil)
		})

		It("retrieves the names of unused volumes", func() {
			unusedVolumes, err := garbageCollector.UnusedVolumes(logger, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(unusedVolumes).To(ConsistOf("sha256ubuntu", "sha256privateubuntu", "unusedLayerVolume", "unusedLocalVolume-timestamp"))
		})

		Context("when certain chain IDs should be preserved", func() {

			It("doesnt list it as unused", func() {
				unusedVolumes, err := garbageCollector.UnusedVolumes(logger, []string{"sha256ubuntu"})
				Expect(err).NotTo(HaveOccurred())

				Expect(unusedVolumes).To(ConsistOf("sha256privateubuntu", "unusedLayerVolume", "unusedLocalVolume-timestamp"))
			})
		})

		Context("when retrieving images fails", func() {
			BeforeEach(func() {
				fakeImageCloner.ImageIDsReturns(nil, errors.New("failed to retrieve images"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger, nil)
				Expect(err).To(MatchError(ContainSubstring("failed to retrieve images")))
			})
		})

		Context("when getting the dependencies of a image fails", func() {
			BeforeEach(func() {
				fakeDependencyManager.DependenciesReturns(nil, errors.New("failed to access deps"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger, nil)
				Expect(err).To(MatchError(ContainSubstring("failed to access deps")))
			})
		})

		Context("when retrieving volume list fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumesReturns(nil, errors.New("failed to retrieve volume list"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger, nil)
				Expect(err).To(MatchError(ContainSubstring("failed to retrieve volume list")))
			})
		})
	})

	Describe("MarkUnused", func() {
		var (
			unusedVolumes []string
		)

		BeforeEach(func() {
			fakeVolumeDriver.VolumePathStub = func(_ lager.Logger, id string) (string, error) {
				return filepath.Join("/store/volumes", id), nil
			}

			fakeVolumeDriver.VolumesReturns([]string{
				"vol-a",
				"vol-b",
				"vol-c",
				"vol-d",
				"vol-e",
				"gc.vol-f",
			}, nil)

			fakeDependencyManager.DependenciesStub = func(id string) ([]string, error) {
				return map[string][]string{
					"image:idA":                         []string{"vol-a", "vol-b"},
					"image:idB":                         []string{"vol-a", "vol-c"},
					"baseimage:docker:///ubuntu":        []string{"vol-d"},
					"baseimage:docker://private/ubuntu": []string{"vol-e"},
				}[id], nil
			}

			fakeImageCloner.ImageIDsReturns([]string{"idA", "idB"}, nil)

			unusedVolumes = []string{"vol-d", "vol-e"}
		})

		It("moves unused volumes to the gc folder", func() {
			Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(Succeed())
			Expect(fakeVolumeDriver.MoveVolumeCallCount()).To(Equal(2))
			_, from1, to1 := fakeVolumeDriver.MoveVolumeArgsForCall(0)
			Expect(from1).To(MatchRegexp("/store/volumes/vol-[ed]"))
			Expect(to1).To(MatchRegexp("/store/volumes/gc.vol-[ed]"))

			_, from2, to2 := fakeVolumeDriver.MoveVolumeArgsForCall(1)
			Expect(from2).To(MatchRegexp("/store/volumes/vol-[ed]"))
			Expect(to2).To(MatchRegexp("/store/volumes/gc.vol-[ed]"))

			Expect(from1).ToNot(Equal(from2))
			Expect(to1).ToNot(Equal(to2))
		})

		It("doesn't remark volumes for gc", func() {
			Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(Succeed())

			for i := 0; i < fakeVolumeDriver.MoveVolumeCallCount(); i++ {
				_, from, _ := fakeVolumeDriver.MoveVolumeArgsForCall(i)
				Expect(from).NotTo(Equal("/store/volumes/gc.vol-f"))
			}
		})

		Context("when checking the volume path fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumePathStub = func(_ lager.Logger, id string) (string, error) {
					if id == "vol-d" {
						return "", errors.New("volume path failed")
					}

					return filepath.Join("/store/volumes", id), nil
				}
			})

			It("returns an error", func() {
				Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(MatchError(ContainSubstring("1/2 volumes failed to be marked as unused")))
			})

			It("still tries to move the other unused volumes", func() {
				Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(HaveOccurred())
				Expect(fakeVolumeDriver.MoveVolumeCallCount()).To(Equal(1))
			})
		})

		Context("when moving volumes fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.MoveVolumeReturns(errors.New("Failed to move"))
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

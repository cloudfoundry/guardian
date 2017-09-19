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

		garbageCollector = garbage_collector.NewGC(fakeVolumeDriver, fakeImageCloner, fakeDependencyManager)

		logger = lagertest.NewTestLogger("garbage_collector")
	})

	Describe("UnusedVolumes", func() {
		BeforeEach(func() {
			fakeVolumeDriver.VolumePathStub = func(_ lager.Logger, id string) (string, error) {
				return filepath.Join("/store/volumes", id), nil
			}

			fakeVolumeDriver.VolumesReturns([]string{
				"sha256:vol-a",
				"sha256:vol-b",
				"sha256:vol-c",
				"sha256:vol-d",
				"sha256:vol-e",
				"gc.sha256:vol-f",
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

		It("retrieves the names of unused volumes", func() {
			unusedVolumes, err := garbageCollector.UnusedVolumes(logger, []string{})
			Expect(err).NotTo(HaveOccurred())

			Expect(unusedVolumes).To(ConsistOf("sha256:vol-d", "sha256:vol-e"))
		})

		Context("when a list of images to keep is provided", func() {
			It("doesn't mark them for collection", func() {
				unusedVolumes, err := garbageCollector.UnusedVolumes(logger, []string{"docker:///ubuntu"})
				Expect(err).NotTo(HaveOccurred())

				Expect(unusedVolumes).To(ConsistOf("sha256:vol-e"))
			})

			Context("when the image to keep is from a private registry", func() {
				It("doesn't mark them for collection", func() {
					unusedVolumes, err := garbageCollector.UnusedVolumes(logger, []string{"docker://private/ubuntu"})
					Expect(err).NotTo(HaveOccurred())

					Expect(unusedVolumes).To(ConsistOf("sha256:vol-d"))
				})
			})
		})

		Context("when retrieving images fails", func() {
			BeforeEach(func() {
				fakeImageCloner.ImageIDsReturns(nil, errors.New("failed to retrieve images"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger, []string{})
				Expect(err).To(MatchError(ContainSubstring("failed to retrieve images")))
			})
		})

		Context("when getting the dependencies of a image fails", func() {
			BeforeEach(func() {
				fakeDependencyManager.DependenciesReturns(nil, errors.New("failed to access deps"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger, []string{})
				Expect(err).To(MatchError(ContainSubstring("failed to access deps")))
			})
		})

		Context("when retrieving volume list fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumesReturns(nil, errors.New("failed to retrieve volume list"))
			})

			It("returns an error", func() {
				_, err := garbageCollector.UnusedVolumes(logger, []string{})
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
				"sha256:vol-a",
				"sha256:vol-b",
				"sha256:vol-c",
				"sha256:vol-d",
				"sha256:vol-e",
				"gc.sha256:vol-f",
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

			unusedVolumes = []string{"sha256:vol-d", "sha256:vol-e"}
		})

		It("moves unused volumes to the gc folder", func() {
			Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(Succeed())
			Expect(fakeVolumeDriver.MoveVolumeCallCount()).To(Equal(2))
			_, from1, to1 := fakeVolumeDriver.MoveVolumeArgsForCall(0)
			Expect(from1).To(MatchRegexp("/store/volumes/sha256:vol-[ed]"))
			Expect(to1).To(MatchRegexp("/store/volumes/gc.sha256:vol-[ed]"))

			_, from2, to2 := fakeVolumeDriver.MoveVolumeArgsForCall(1)
			Expect(from2).To(MatchRegexp("/store/volumes/sha256:vol-[ed]"))
			Expect(to2).To(MatchRegexp("/store/volumes/gc.sha256:vol-[ed]"))

			Expect(from1).ToNot(Equal(from2))
			Expect(to1).ToNot(Equal(to2))
		})

		It("doesn't remark volumes for gc", func() {
			Expect(garbageCollector.MarkUnused(logger, unusedVolumes)).To(Succeed())

			for i := 0; i < fakeVolumeDriver.MoveVolumeCallCount(); i++ {
				_, from, _ := fakeVolumeDriver.MoveVolumeArgsForCall(i)
				Expect(from).NotTo(Equal("/store/volumes/gc.sha256:vol-f"))
			}
		})

		Context("when checking the volume path fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.VolumePathStub = func(_ lager.Logger, id string) (string, error) {
					if id == "sha256:vol-d" {
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
				"sha256:vol-a",
				"gc.sha256:vol-b",
				"gc.sha256:vol-c",
				"sha256:vol-d",
				"sha256:vol-e",
				"gc.sha256:vol-f",
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

			Expect(volumes).To(ContainElement("gc.sha256:vol-b"))
			Expect(volumes).To(ContainElement("gc.sha256:vol-c"))
			Expect(volumes).To(ContainElement("gc.sha256:vol-f"))
		})

		Context("when destroying a volume fails", func() {
			BeforeEach(func() {
				fakeVolumeDriver.DestroyVolumeStub = func(_ lager.Logger, volID string) error {
					if volID == "gc.sha256:vol-f" {
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

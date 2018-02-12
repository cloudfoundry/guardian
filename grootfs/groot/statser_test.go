package groot_test

import (
	"errors"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Statser", func() {
	var (
		fakeImageCloner *grootfakes.FakeImageCloner
		statser         *groot.Statser
		logger          lager.Logger
	)

	BeforeEach(func() {
		fakeImageCloner = new(grootfakes.FakeImageCloner)
		statser = groot.IamStatser(fakeImageCloner)
		logger = lagertest.NewTestLogger("statser")
	})

	Describe("Stats", func() {
		It("asks for stats from the imageCloner", func() {
			fakeImageCloner.StatsReturns(groot.VolumeStats{}, nil)
			_, err := statser.Stats(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeImageCloner.StatsCallCount()).To(Equal(1))
			_, id := fakeImageCloner.StatsArgsForCall(0)
			Expect(id).To(Equal("some-id"))
		})

		It("asks for stats from the imageCloner", func() {
			stats := groot.VolumeStats{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     1024,
					ExclusiveBytesUsed: 512,
				},
			}
			fakeImageCloner.StatsReturns(stats, nil)

			returnedStats, err := statser.Stats(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedStats).To(Equal(stats))
		})

		Context("when imageCloner fails", func() {
			It("returns an error", func() {
				fakeImageCloner.StatsReturns(groot.VolumeStats{}, errors.New("sorry"))

				_, err := statser.Stats(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("sorry")))
			})
		})
	})
})

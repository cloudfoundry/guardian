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

var _ = Describe("Metricser", func() {
	var (
		fakeImageCloner *grootfakes.FakeImageCloner
		metricser       *groot.Metricser
		logger          lager.Logger
	)

	BeforeEach(func() {
		fakeImageCloner = new(grootfakes.FakeImageCloner)
		metricser = groot.IamMetricser(fakeImageCloner)
		logger = lagertest.NewTestLogger("metricser")
	})

	Describe("Metrics", func() {
		It("asks for metrics from the imageCloner", func() {
			fakeImageCloner.MetricsReturns(groot.VolumeMetrics{}, nil)
			_, err := metricser.Metrics(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeImageCloner.MetricsCallCount()).To(Equal(1))
			_, id := fakeImageCloner.MetricsArgsForCall(0)
			Expect(id).To(Equal("some-id"))
		})

		It("asks for metrics from the imageCloner", func() {
			metrics := groot.VolumeMetrics{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     1024,
					ExclusiveBytesUsed: 512,
				},
			}
			fakeImageCloner.MetricsReturns(metrics, nil)

			returnedMetrics, err := metricser.Metrics(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedMetrics).To(Equal(metrics))
		})

		Context("when imageCloner fails", func() {
			It("returns an error", func() {
				fakeImageCloner.MetricsReturns(groot.VolumeMetrics{}, errors.New("sorry"))

				_, err := metricser.Metrics(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("sorry")))
			})
		})
	})
})

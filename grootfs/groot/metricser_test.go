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
		fakeBundler *grootfakes.FakeBundler
		metricser   *groot.Metricser
		logger      lager.Logger
	)

	BeforeEach(func() {
		fakeBundler = new(grootfakes.FakeBundler)
		metricser = groot.IamMetricser(fakeBundler)
		logger = lagertest.NewTestLogger("metricser")
	})

	Describe("Metrics", func() {
		It("asks for metrics from the bundler", func() {
			fakeBundler.MetricsReturns(groot.VolumeMetrics{}, nil)
			_, err := metricser.Metrics(logger, "some-id")
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeBundler.MetricsCallCount()).To(Equal(1))
			_, id := fakeBundler.MetricsArgsForCall(0)
			Expect(id).To(Equal("some-id"))
		})

		It("asks for metrics from the bundler", func() {
			metrics := groot.VolumeMetrics{
				DiskUsage: groot.DiskUsage{
					TotalBytesUsed:     1024,
					ExclusiveBytesUsed: 512,
				},
			}
			fakeBundler.MetricsReturns(metrics, nil)

			returnedMetrics, err := metricser.Metrics(logger, "some-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(returnedMetrics).To(Equal(metrics))
		})

		Context("when bundler fails", func() {
			It("returns an error", func() {
				fakeBundler.MetricsReturns(groot.VolumeMetrics{}, errors.New("sorry"))

				_, err := metricser.Metrics(logger, "some-id")
				Expect(err).To(MatchError(ContainSubstring("sorry")))
			})
		})
	})
})

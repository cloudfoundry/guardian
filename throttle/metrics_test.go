package throttle_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/guardian/throttle/throttlefakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerMetricsSource", func() {

	var (
		metricsSource     throttle.MetricsSource
		containerManager  *throttlefakes.FakeContainerManager
		logger            *lagertest.TestLogger
		metrics           map[string]gardener.ActualContainerMetrics
		collectMetricsErr error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("container-metrics-test")
		containerManager = new(throttlefakes.FakeContainerManager)
		containerManager.HandlesReturns([]string{"foo", "bar"}, nil)
		containerManager.MetricsReturnsOnCall(0, containerMetric(1, 2), nil)
		containerManager.MetricsReturnsOnCall(1, containerMetric(3, 4), nil)
		metricsSource = throttle.NewContainerMetricsSource(containerManager)
	})

	JustBeforeEach(func() {
		metrics, collectMetricsErr = metricsSource.CollectMetrics(logger)
	})

	It("collects metrics", func() {
		Expect(collectMetricsErr).ToNot(HaveOccurred())
		Expect(metrics).To(HaveLen(2))
		Expect(metrics).To(HaveKeyWithValue("foo", containerMetric(1, 2)))
		Expect(metrics).To(HaveKeyWithValue("bar", containerMetric(3, 4)))
	})

	When("listing container handles fails", func() {
		BeforeEach(func() {
			containerManager.HandlesReturns(nil, errors.New("list-containers-err"))
		})

		It("returns the error", func() {
			Expect(collectMetricsErr).To(MatchError("list-containers-err"))
		})
	})

	When("getting metrics for a handle fails", func() {
		BeforeEach(func() {
			containerManager.MetricsReturnsOnCall(0, gardener.ActualContainerMetrics{}, errors.New("metrics-err"))
		})

		It("continues without failing", func() {
			Expect(collectMetricsErr).NotTo(HaveOccurred())
			Expect(metrics).To(HaveLen(1))
			Expect(metrics).To(HaveKeyWithValue("bar", containerMetric(3, 4)))
		})
	})
})

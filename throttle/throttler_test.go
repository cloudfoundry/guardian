package throttle_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/guardian/throttle/throttlefakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
)

var _ = Describe("Throttler", func() {
	var (
		logger        *lagertest.TestLogger
		metricsSource *throttlefakes.FakeMetricsSource
		enforcer      *throttlefakes.FakeEnforcer
		throttler     throttle.Throttler
		throttleErr   error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("throttler-test")
		metricsSource = new(throttlefakes.FakeMetricsSource)
		enforcer = new(throttlefakes.FakeEnforcer)
		throttler = throttle.NewThrottler(metricsSource, enforcer)
	})

	JustBeforeEach(func() {
		throttleErr = throttler.Run(logger)
	})

	It("does not fail", func() {
		Expect(throttleErr).NotTo(HaveOccurred())
	})

	It("invokes the metrics source", func() {
		Expect(metricsSource.CollectMetricsCallCount()).To(Equal(1))
	})

	It("logs that throttling has begun and has ended", func() {
		logs := logger.LogMessages()
		Expect(logs).To(ContainElement(ContainSubstring("throttle.starting")))
		Expect(logs).To(ContainElement(ContainSubstring("throttle.finished")))
	})

	When("getting the metrics fails", func() {
		BeforeEach(func() {
			metricsSource.CollectMetricsReturns(nil, errors.New("metrics-err"))
		})

		It("returns the error", func() {
			Expect(throttleErr).To(MatchError("metrics-err"))
		})
	})

	When("an app is above entitlement", func() {
		BeforeEach(func() {
			metricsSource.CollectMetricsReturns(map[string]gardener.ActualContainerMetrics{
				"bar": containerMetric(120, 100),
			}, nil)
		})

		It("is punished", func() {
			Expect(enforcer.PunishCallCount()).To(Equal(1))
			_, actualHandle := enforcer.PunishArgsForCall(0)
			Expect(actualHandle).To(Equal("bar"))
			Expect(enforcer.ReleaseCallCount()).To(Equal(0))
		})
	})

	When("the punisher fails to punish an app", func() {
		BeforeEach(func() {
			metricsSource.CollectMetricsReturns(map[string]gardener.ActualContainerMetrics{
				"foo": containerMetric(120, 100),
				"bar": containerMetric(150, 100),
				"baz": containerMetric(200, 100),
			}, nil)
			enforcer.PunishReturnsOnCall(0, errors.New("first-failure"))
			enforcer.PunishReturnsOnCall(1, errors.New("second-failure"))
		})

		It("returns a multi error", func() {
			Expect(throttleErr).To(MatchError(And(ContainSubstring("first-failure"), ContainSubstring("second-failure"))))
		})
	})

	When("an app is below entitlement", func() {
		BeforeEach(func() {
			metricsSource.CollectMetricsReturns(map[string]gardener.ActualContainerMetrics{
				"bar": containerMetric(50, 100),
			}, nil)
		})

		It("is released", func() {
			Expect(enforcer.PunishCallCount()).To(Equal(0))
			Expect(enforcer.ReleaseCallCount()).To(Equal(1))
			_, actualHandle := enforcer.ReleaseArgsForCall(0)
			Expect(actualHandle).To(Equal("bar"))
		})
	})

})

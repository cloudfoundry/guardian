package metrics_test

import (
	"time"

	"code.cloudfoundry.org/guardian/metrics"
	fakes "code.cloudfoundry.org/guardian/metrics/metricsfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	dropsonde_metrics "github.com/cloudfoundry/dropsonde/metrics"
	"github.com/pivotal-golang/clock/fakeclock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PeriodicMetronNotifier", func() {
	var (
		sender *fake.FakeMetricSender

		fakeMetrics    *fakes.FakeMetrics
		reportInterval time.Duration
		fakeClock      *fakeclock.FakeClock

		pmn *metrics.PeriodicMetronNotifier
	)

	BeforeEach(func() {
		reportInterval = 100 * time.Millisecond

		fakeMetrics = new(fakes.FakeMetrics)
		fakeMetrics.LoopDevicesReturns(33)
		fakeMetrics.BackingStoresReturns(12)
		fakeMetrics.DepotDirsReturns(3)

		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

		sender = fake.NewFakeMetricSender()
		dropsonde_metrics.Initialize(sender, nil)
	})

	JustBeforeEach(func() {
		pmn = metrics.NewPeriodicMetronNotifier(
			lagertest.NewTestLogger("test"),
			fakeMetrics,
			reportInterval,
			fakeClock,
		)
		pmn.Start()
	})

	AfterEach(func() {
		pmn.Stop()
	})

	Context("when the report interval elapses", func() {
		It("emits metrics", func() {
			fakeClock.Increment(reportInterval)

			Eventually(func() fake.Metric {
				return sender.GetValue("LoopDevices")
			}).Should(Equal(fake.Metric{
				Value: 33,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("BackingStores")
			}).Should(Equal(fake.Metric{
				Value: 12,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("DepotDirs")
			}).Should(Equal(fake.Metric{
				Value: 3,
				Unit:  "Metric",
			}))
		})
	})
})

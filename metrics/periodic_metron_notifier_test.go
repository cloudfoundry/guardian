package metrics_test

import (
	"time"

	"code.cloudfoundry.org/guardian/metrics"
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

		testMetrics    metrics.Metrics
		reportInterval time.Duration
		clock          *fakeclock.FakeClock

		pmn *metrics.PeriodicMetronNotifier
	)

	BeforeEach(func() {
		reportInterval = 100 * time.Millisecond

		testMetrics = map[string]func() int{
			"fooMetric": func() int { return 1 },
			"barMetric": func() int { return 2 },
		}

		clock = fakeclock.NewFakeClock(time.Unix(123, 456))

		sender = fake.NewFakeMetricSender()
		dropsonde_metrics.Initialize(sender, nil)
	})

	JustBeforeEach(func() {
		pmn = metrics.NewPeriodicMetronNotifier(
			lagertest.NewTestLogger("test"),
			testMetrics,
			reportInterval,
			clock,
		)
		pmn.Start()
	})

	AfterEach(func() {
		pmn.Stop()
	})

	Context("when the report interval elapses", func() {
		It("emits metrics", func() {
			clock.Increment(reportInterval)

			Eventually(func() fake.Metric {
				return sender.GetValue("fooMetric")
			}).Should(Equal(fake.Metric{
				Value: 1,
				Unit:  "Metric",
			}))

			Eventually(func() fake.Metric {
				return sender.GetValue("barMetric")
			}).Should(Equal(fake.Metric{
				Value: 2,
				Unit:  "Metric",
			}))
		})
	})
})

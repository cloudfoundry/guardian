package metrics_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Emitter", func() {
	var (
		fakeMetronPort     uint16
		fakeMetron         *testhelpers.FakeMetron
		fakeMetronClosed   chan struct{}
		fakeMetronEndpoint string
		emitter            *metrics.Emitter
		logger             *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeMetronPort = uint16(5000 + GinkgoParallelNode())

		fakeMetron = testhelpers.NewFakeMetron(fakeMetronPort)
		Expect(fakeMetron.Listen()).To(Succeed())

		fakeMetronEndpoint = fmt.Sprintf("127.0.0.1:%d", fakeMetronPort)

		logger = lagertest.NewTestLogger("emitter")
		emitter = metrics.NewEmitter(logger, fakeMetronEndpoint)

		fakeMetronClosed = make(chan struct{})
		go func() {
			defer GinkgoRecover()
			Expect(fakeMetron.Run()).To(Succeed())
			close(fakeMetronClosed)
		}()

	})

	AfterEach(func() {
		Expect(fakeMetron.Stop()).To(Succeed())
		Eventually(fakeMetronClosed).Should(BeClosed())
	})

	Describe("TryEmitUsage", func() {
		It("emits metrics", func() {
			emitter.TryEmitUsage(logger, "foo", 1000, "units")

			var fooMetrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				fooMetrics = fakeMetron.ValueMetricsFor("foo")
				return fooMetrics
			}).Should(HaveLen(1))

			Expect(*fooMetrics[0].Name).To(Equal("foo"))
			Expect(*fooMetrics[0].Unit).To(Equal("units"))
			Expect(*fooMetrics[0].Value).To(Equal(float64(1000)))
		})
	})

	Describe("TryEmitDurationFrom", func() {
		It("emits metrics", func() {
			from := time.Now().Add(-1 * time.Second)
			emitter.TryEmitDurationFrom(logger, "foo", from)

			var fooMetrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				fooMetrics = fakeMetron.ValueMetricsFor("foo")
				return fooMetrics
			}).Should(HaveLen(1))

			Expect(*fooMetrics[0].Name).To(Equal("foo"))
			Expect(*fooMetrics[0].Unit).To(Equal("nanos"))
			Expect(*fooMetrics[0].Value).To(SatisfyAll(
				BeNumerically(">", float64(time.Second-time.Millisecond)),
				BeNumerically("<", float64(time.Second+10*time.Millisecond)),
			))
		})
	})
})

package metrics_test

import (
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Emitter", func() {
	var (
		fakeMetronPort   uint16
		fakeMetron       *testhelpers.FakeMetron
		fakeMetronClosed chan struct{}
		emitter          *metrics.Emitter
		logger           *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeMetronPort = uint16(5000 + GinkgoParallelNode())

		fakeMetron = testhelpers.NewFakeMetron(fakeMetronPort)
		Expect(fakeMetron.Listen()).To(Succeed())

		Expect(
			dropsonde.Initialize(fmt.Sprintf("127.0.0.1:%d", fakeMetronPort), "foo"),
		).To(Succeed())

		fakeMetronClosed = make(chan struct{})
		go func() {
			defer GinkgoRecover()
			Expect(fakeMetron.Run()).To(Succeed())
			close(fakeMetronClosed)
		}()

		emitter = metrics.NewEmitter()

		logger = lagertest.NewTestLogger("emitter")
	})

	AfterEach(func() {
		Expect(fakeMetron.Stop()).To(Succeed())
		Eventually(fakeMetronClosed).Should(BeClosed())
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
				BeNumerically("<", float64(time.Second+time.Millisecond)),
			))
		})
	})

	Describe("TryEmitError", func() {
		It("emits error", func() {
			emitter.TryEmitError(logger, "create", errors.New("hello"), int32(10))

			var errors []events.Error
			Eventually(func() []events.Error {
				errors = fakeMetron.Errors()
				return errors
			}).Should(HaveLen(1))

			Expect(*errors[0].Source).To(Equal("grootfs.create"))
			Expect(*errors[0].Code).To(Equal(int32(10)))
			Expect(*errors[0].Message).To(Equal("hello"))
		})
	})
})

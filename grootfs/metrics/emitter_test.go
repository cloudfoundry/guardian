package metrics_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/grootfs/metrics"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Emitter", func() {
	Describe("NewEmitter", func() {
		Context("when the metron endpoint is not provided", func() {
			It("returns an error", func() {
				_, err := metrics.NewEmitter("")
				Expect(err).To(MatchError(ContainSubstring("destination variable not set")))
			})
		})
	})

	Describe("EmitDuration", func() {
		var (
			fakeMetronPort   uint16
			fakeMetron       *testhelpers.FakeMetron
			fakeMetronClosed chan struct{}
			emitter          *metrics.Emitter
		)

		BeforeEach(func() {
			fakeMetronPort = uint16(5000 + GinkgoParallelNode())

			fakeMetron = testhelpers.NewFakeMetron(fakeMetronPort)
			Expect(fakeMetron.Listen()).To(Succeed())

			var err error
			emitter, err = metrics.NewEmitter(
				fmt.Sprintf("127.0.0.1:%d", fakeMetronPort),
			)
			Expect(err).NotTo(HaveOccurred())

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

		It("emits metrics", func() {
			Expect(emitter.EmitDuration("foo", time.Second)).To(Succeed())

			var fooMetrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				fooMetrics = fakeMetron.ValueMetricsFor("foo")
				return fooMetrics
			}).Should(HaveLen(1))

			Expect(*fooMetrics[0].Name).To(Equal("foo"))
			Expect(*fooMetrics[0].Unit).To(Equal("nanos"))
			Expect(*fooMetrics[0].Value).To(Equal(float64(time.Second)))
		})
	})
})

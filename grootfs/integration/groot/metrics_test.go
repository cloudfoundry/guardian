package groot_test

import (
	"io"
	"net"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Metrics", func() {
	var (
		fakeMetronPort   uint16
		fakeMetron       *testhelpers.FakeMetron
		fakeMetronClosed chan struct{}
	)

	BeforeEach(func() {
		fakeMetronPort = uint16(5000 + GinkgoParallelNode())

		fakeMetron = testhelpers.NewFakeMetron(fakeMetronPort)
		Expect(fakeMetron.Listen()).To(Succeed())

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

	Describe("Create", func() {
		It("emits the total creation time", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Create(groot.CreateSpec{
					ID:        "my-id",
					BaseImage: "docker:///cfgarden/empty:v0.1.0",
				})
			Expect(err).NotTo(HaveOccurred())

			var imageCreationTimeMetrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				imageCreationTimeMetrics = fakeMetron.ValueMetricsFor("ImageCreationTime")
				return imageCreationTimeMetrics
			}).Should(HaveLen(1))

			Expect(*imageCreationTimeMetrics[0].Name).To(Equal("ImageCreationTime"))
			Expect(*imageCreationTimeMetrics[0].Unit).To(Equal("nanos"))
			Expect(*imageCreationTimeMetrics[0].Value).NotTo(BeZero())
		})

		Context("when the metron endpoint is not provided", func() {
			It("complaints but succeeds", func() {
				buffer := gbytes.NewBuffer()

				_, err := Runner.
					WithStderr(io.MultiWriter(GinkgoWriter, buffer)).
					Create(groot.CreateSpec{
						ID:        "my-id",
						BaseImage: "docker:///cfgarden/empty:v0.1.0",
					})
				Expect(err).NotTo(HaveOccurred())

				Eventually(buffer).Should(gbytes.Say("failed-to-initialize-metrics-emitter"))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			_, err := Runner.
				Create(groot.CreateSpec{
					ID:        "my-id",
					BaseImage: "docker:///cfgarden/empty:v0.1.0",
				})
			Expect(err).NotTo(HaveOccurred())
		})

		It("emits the total deletion time", func() {
			err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Delete("my-id")
			Expect(err).NotTo(HaveOccurred())

			var imageDeletionTimeMetrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				imageDeletionTimeMetrics = fakeMetron.ValueMetricsFor("ImageDeletionTime")
				return imageDeletionTimeMetrics
			}).Should(HaveLen(1))

			Expect(*imageDeletionTimeMetrics[0].Name).To(Equal("ImageDeletionTime"))
			Expect(*imageDeletionTimeMetrics[0].Unit).To(Equal("nanos"))
			Expect(*imageDeletionTimeMetrics[0].Value).NotTo(BeZero())
		})
	})
})

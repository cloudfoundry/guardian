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

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageCreationTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageCreationTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
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

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageDeletionTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageDeletionTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})
	})

	Describe("Stats", func() {
		BeforeEach(func() {
			_, err := Runner.
				Create(groot.CreateSpec{
					ID:        "my-id",
					BaseImage: "docker:///cfgarden/empty:v0.1.0",
				})
			Expect(err).NotTo(HaveOccurred())
		})

		It("emits the total time for metrics command", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Stats("my-id")
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageStatsTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageStatsTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})
	})

	Describe("Clean", func() {
		BeforeEach(func() {
			_, err := Runner.
				Create(groot.CreateSpec{
					ID:        "my-id",
					BaseImage: "docker:///cfgarden/empty:v0.1.0",
				})
			Expect(err).NotTo(HaveOccurred())
		})

		It("emits the total clean time", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Clean(0, []string{})
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageCleanTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageCleanTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})
	})
})

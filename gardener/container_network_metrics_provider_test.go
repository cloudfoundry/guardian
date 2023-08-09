package gardener_test

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewNoopContainerNetworkMetricsProvider", func() {
	var (
		logger                 lager.Logger
		networkMetricsProvider *gardener.NoopContainerNetworkMetricsProvider
	)

	BeforeEach(func() {
		networkMetricsProvider = gardener.NewNoopContainerNetworkMetricsProvider()
		logger = lagertest.NewTestLogger("test")
	})

	Describe("Get", func() {

		It("should return nil", func() {
			actualNetworkMetrics, err := networkMetricsProvider.Get(logger, "random-handle")

			Expect(err).ToNot(HaveOccurred())
			Expect(actualNetworkMetrics).To(BeNil())
		})

	})

})

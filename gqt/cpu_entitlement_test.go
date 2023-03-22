package gqt_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/sysinfo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CPU entitlement", func() {
	var (
		client        *runner.RunningGarden
		container     garden.Container
		containerSpec garden.ContainerSpec
	)

	BeforeEach(func() {
		containerSpec = garden.ContainerSpec{
			Limits: garden.Limits{
				CPU: garden.CPULimits{
					Weight: 100,
				},
			},
		}
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
		var err error
		container, err = client.Create(containerSpec)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("defaults to optimal CPU entitlement per share", func() {
		actualCpuEntitlementPerShare := getCpuEntitlementPerShare(container, containerSpec.Limits.CPU.Weight)

		resourcesProvider := sysinfo.NewResourcesProvider(config.DepotDir)
		totalMemory, err := resourcesProvider.TotalMemory()
		Expect(err).NotTo(HaveOccurred())

		memoryInMb := float64(totalMemory) / float64(1024*1024)
		cpuCores, err := resourcesProvider.CPUCores()
		Expect(err).NotTo(HaveOccurred())
		expectedCpuEntitlementPerShare := float64(cpuCores*100) / memoryInMb

		Expect(actualCpuEntitlementPerShare).To(BeNumerically("~", expectedCpuEntitlementPerShare, 0.0001))
	})

	Context("when CPU entitlement per share is set", func() {
		BeforeEach(func() {
			config.CPUEntitlementPerShare = float64ptr(3.45)
		})

		It("uses it", func() {
			actualCpuEntitlementPerShare := getCpuEntitlementPerShare(container, containerSpec.Limits.CPU.Weight)
			Expect(actualCpuEntitlementPerShare).To(BeNumerically("~", *config.CPUEntitlementPerShare, 0.01))
		})
	})

})

func getCpuEntitlementPerShare(container garden.Container, shares uint64) float64 {
	metrics := getMetrics(container)
	return float64(100*metrics.CPUEntitlement) / float64(shares*uint64(metrics.Age.Nanoseconds()))
}

func getMetrics(container garden.Container) garden.Metrics {
	metrics, err := container.Metrics()
	Expect(err).NotTo(HaveOccurred())
	return metrics
}

func float64ptr(f float64) *float64 {
	return &f
}

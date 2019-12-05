package throttle_test

import (
	"testing"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestThrottler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Throttler Suite")
}

func containerMetric(cpuUsage, cpuEntitlement uint64) gardener.ActualContainerMetrics {
	return gardener.ActualContainerMetrics{
		StatsContainerMetrics: gardener.StatsContainerMetrics{
			CPU: garden.ContainerCPUStat{Usage: cpuUsage},
		},
		CPUEntitlement: cpuEntitlement,
	}
}

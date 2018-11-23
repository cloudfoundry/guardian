package runrunc

import (
	"time"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager"
)

type RuncMetricsCollector struct {
	statser *Statser
}

func NewRuncMetricsCollector(statser *Statser) *RuncMetricsCollector {
	return &RuncMetricsCollector{
		statser: statser,
	}
}

func (c *RuncMetricsCollector) Collect(log lager.Logger, handles []string) (map[string]gardener.ActualContainerMetrics, error) {
	result := make(map[string]gardener.ActualContainerMetrics)
	for _, h := range handles {
		m, err := c.collectSingleContainerMetric(log, h)
		if err != nil {
			return nil, err
		}
		result[h] = m
	}

	return result, nil
}

func (c *RuncMetricsCollector) collectSingleContainerMetric(log lager.Logger, handle string) (gardener.ActualContainerMetrics, error) {
	containerMetrics, err := c.statser.Stats(log, handle)
	if err != nil {
		return gardener.ActualContainerMetrics{}, err
	}

	return gardener.ActualContainerMetrics{
		StatsContainerMetrics: containerMetrics,
		CPUEntitlement: calculateEntitlement(containerMetrics.Memory.HierarchicalMemoryLimit,
			containerMetrics.Age),
	}, nil
}

func calculateEntitlement(memoryLimitInBytes uint64, containerAge time.Duration) uint64 {
	return uint64(gigabytes(memoryLimitInBytes) * float64(containerAge.Nanoseconds()))
}

func gigabytes(bytes uint64) float64 {
	return float64(bytes) / (1024 * 1024 * 1024)
}

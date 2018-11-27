package runcontainerd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	"github.com/containerd/cgroups"
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/typeurl"
)

type runcState struct {
	Created time.Time `json:"created"`
}
type NerdMetrics interface {
	Metrics(log lager.Logger, containerHandle string) (*types.Metric, error)
}

type ContainerdMetricsCollector struct {
	nerd       NerdMetrics
	runcBinary goci.RuncBinary
}

func NewContainerdMetricsCollector(nerd NerdMetrics, runcBinary goci.RuncBinary) *ContainerdMetricsCollector {
	return &ContainerdMetricsCollector{
		nerd:       nerd,
		runcBinary: runcBinary,
	}
}

func (c *ContainerdMetricsCollector) Collect(log lager.Logger, handles []string) (map[string]gardener.ActualContainerMetrics, error) {
	result := make(map[string]gardener.ActualContainerMetrics)
	for _, h := range handles {
		m, err := c.nerd.Metrics(log, h)
		if err != nil {
			return nil, err
		}

		cgroupMetrics, err := toCgroupMetrics(m)
		if err != nil {
			return nil, err
		}

		createdAt, err := c.getContainerCreationTime(h)
		if err != nil {
			return nil, err
		}
		containerAge := time.Since(createdAt)

		result[h] = toGardenMetrics(cgroupMetrics, containerAge)
	}

	return result, nil
}

func (c *ContainerdMetricsCollector) getContainerCreationTime(id string) (time.Time, error) {
	buf := new(bytes.Buffer)
	stateCommand := c.runcBinary.StateCommand(id, "/tmp/runcfoo.log")
	stateCommand.Stdout = buf
	if err := stateCommand.Run(); err != nil {
		return time.Now(), fmt.Errorf("runC state: %s", err)
	}

	var stateData runcState
	if err := json.NewDecoder(buf).Decode(&stateData); err != nil {
		return time.Now(), fmt.Errorf("decode state: %s", err)
	}
	return stateData.Created, nil
}

func toCgroupMetrics(nerdMetrics *types.Metric) (*cgroups.Metrics, error) {
	anydata, err := typeurl.UnmarshalAny(nerdMetrics.Data)
	if err != nil {
		return nil, err
	}
	data, ok := anydata.(*cgroups.Metrics)
	if !ok {
		return nil, fmt.Errorf("cannot convert metric data to cgroups.Metrics: %v", anydata)
	}
	return data, nil
}

func toGardenMetrics(nerdMetrics *cgroups.Metrics, containerAge time.Duration) gardener.ActualContainerMetrics {

	statsContainerMetrics := toStatsContainerMetric(nerdMetrics, containerAge)
	return gardener.ActualContainerMetrics{
		StatsContainerMetrics: statsContainerMetrics,
		CPUEntitlement: calculateEntitlement(statsContainerMetrics.Memory.HierarchicalMemoryLimit,
			statsContainerMetrics.Age),
	}

}

func toStatsContainerMetric(cgroupMetrics *cgroups.Metrics, containerAge time.Duration) gardener.StatsContainerMetrics {
	return gardener.StatsContainerMetrics{
		CPU:    toContainerCPUStat(cgroupMetrics),
		Memory: toContainerMemoryStat(cgroupMetrics),
		Pid:    toContainerPidStat(cgroupMetrics),
		Age:    containerAge,
	}
}

func toContainerCPUStat(cgroupMetrics *cgroups.Metrics) garden.ContainerCPUStat {
	return garden.ContainerCPUStat{
		Usage:  cgroupMetrics.CPU.Usage.Total,
		User:   cgroupMetrics.CPU.Usage.User,
		System: cgroupMetrics.CPU.Usage.Kernel,
	}
}

func toContainerMemoryStat(cgroupMetrics *cgroups.Metrics) garden.ContainerMemoryStat {
	mstat := cgroupMetrics.Memory
	return garden.ContainerMemoryStat{
		ActiveAnon:              mstat.ActiveAnon,
		ActiveFile:              mstat.ActiveFile,
		Cache:                   mstat.Cache,
		HierarchicalMemoryLimit: mstat.HierarchicalMemoryLimit,
		InactiveAnon:            mstat.InactiveAnon,
		InactiveFile:            mstat.InactiveFile,
		MappedFile:              mstat.MappedFile,
		Pgfault:                 mstat.PgFault,
		Pgmajfault:              mstat.PgMajFault,
		Pgpgin:                  mstat.PgPgIn,
		Pgpgout:                 mstat.PgPgOut,
		Rss:                     mstat.RSS,
		TotalActiveAnon:         mstat.TotalActiveAnon,
		TotalActiveFile:         mstat.TotalActiveFile,
		TotalCache:              mstat.TotalCache,
		TotalInactiveAnon:       mstat.TotalInactiveAnon,
		TotalInactiveFile:       mstat.TotalInactiveFile,
		TotalMappedFile:         mstat.TotalMappedFile,
		TotalPgfault:            mstat.TotalPgFault,
		TotalPgmajfault:         mstat.TotalPgMajFault,
		TotalPgpgin:             mstat.TotalPgPgIn,
		TotalPgpgout:            mstat.TotalPgPgOut,
		TotalRss:                mstat.TotalRSS,
		TotalUnevictable:        mstat.TotalUnevictable,
		Unevictable:             mstat.Unevictable,
		Swap:                    mstat.Swap.Usage,
		HierarchicalMemswLimit:  mstat.HierarchicalSwapLimit,
		TotalSwap:               mstat.Swap.Max,
		TotalUsageTowardLimit:   mstat.TotalRSS + (mstat.TotalCache - mstat.TotalInactiveFile),
	}
}

func toContainerPidStat(cgroupMetrics *cgroups.Metrics) garden.ContainerPidStat {
	return garden.ContainerPidStat{
		Current: cgroupMetrics.Pids.Current,
		Max:     cgroupMetrics.Pids.Limit,
	}
}

func calculateEntitlement(memoryLimitInBytes uint64, containerAge time.Duration) uint64 {
	return uint64(gigabytes(memoryLimitInBytes) * float64(containerAge.Nanoseconds()))
}

func gigabytes(bytes uint64) float64 {
	return float64(bytes) / (1024 * 1024 * 1024)
}

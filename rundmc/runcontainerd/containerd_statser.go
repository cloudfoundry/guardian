//go:build !windows

package runcontainerd

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager/v3"
	cgroup2stats "github.com/containerd/cgroups/v3/cgroup2/stats"
	"github.com/containerd/typeurl/v2"
)

// ContainerMetricsGetter abstracts the containerd task metrics retrieval.
type ContainerMetricsGetter interface {
	TaskMetrics(log lager.Logger, containerID string) (*cgroup2stats.Metrics, time.Time, error)
}

// ContainerdStatser implements the Statser interface using containerd task
// metrics API instead of shelling out to runc. This is required for non-runc
// runtimes (e.g. gVisor/runsc) where "runc events --stats" does not work
// because runc has no knowledge of the container.
type ContainerdStatser struct {
	metricsGetter ContainerMetricsGetter
}

func NewContainerdStatser(metricsGetter ContainerMetricsGetter) *ContainerdStatser {
	return &ContainerdStatser{metricsGetter: metricsGetter}
}

func (s *ContainerdStatser) Stats(log lager.Logger, id string) (gardener.StatsContainerMetrics, error) {
	log = log.Session("containerd-statser", lager.Data{"id": id})

	metrics, createdAt, err := s.metricsGetter.TaskMetrics(log, id)
	if err != nil {
		return gardener.StatsContainerMetrics{}, fmt.Errorf("containerd task metrics: %w", err)
	}

	var memoryStat garden.ContainerMemoryStat
	if metrics.Memory != nil {
		memoryStat = garden.ContainerMemoryStat{
			Anon:         metrics.Memory.Anon,
			File:         metrics.Memory.File,
			ActiveAnon:   metrics.Memory.ActiveAnon,
			InactiveAnon: metrics.Memory.InactiveAnon,
			ActiveFile:   metrics.Memory.ActiveFile,
			InactiveFile: metrics.Memory.InactiveFile,
			Unevictable:  metrics.Memory.Unevictable,
			MappedFile:   metrics.Memory.FileMapped,
			Pgfault:      metrics.Memory.Pgfault,
			Pgmajfault:   metrics.Memory.Pgmajfault,
			SwapCached:   0, // cgroupsv2 does not expose swapcached separately
			// cgroupsv2 does not have hierarchical_memory_limit;
			// usage_limit is the closest equivalent
			HierarchicalMemoryLimit: metrics.Memory.UsageLimit,
			// Populate TotalXxx with the same values (cgroupsv2 unified hierarchy
			// means there is no separate "total" vs "local" distinction)
			TotalActiveAnon:   metrics.Memory.ActiveAnon,
			TotalActiveFile:   metrics.Memory.ActiveFile,
			TotalInactiveAnon: metrics.Memory.InactiveAnon,
			TotalInactiveFile: metrics.Memory.InactiveFile,
			TotalUnevictable:  metrics.Memory.Unevictable,
			TotalMappedFile:   metrics.Memory.FileMapped,
			TotalPgfault:      metrics.Memory.Pgfault,
			TotalPgmajfault:   metrics.Memory.Pgmajfault,
			// For cgroupsv2, "Rss" is essentially Anon + swap-backed anon
			Rss:      metrics.Memory.Anon,
			TotalRss: metrics.Memory.Anon,
			// Cache = file-backed pages
			Cache:      metrics.Memory.File,
			TotalCache: metrics.Memory.File,
			// Swap
			Swap:      metrics.Memory.SwapUsage,
			TotalSwap: metrics.Memory.SwapUsage,
		}
		// Calculate TotalUsageTowardLimit the same way garden does for cgroupsv2:
		// File + Anon + SwapCached - InactiveFile
		totalMemoryUsage := memoryStat.File + memoryStat.Anon + memoryStat.SwapCached
		if memoryStat.InactiveFile > totalMemoryUsage {
			memoryStat.TotalUsageTowardLimit = 0
		} else {
			memoryStat.TotalUsageTowardLimit = totalMemoryUsage - memoryStat.InactiveFile
		}
	}

	var cpuStat garden.ContainerCPUStat
	if metrics.CPU != nil {
		// cgroupsv2 reports CPU in microseconds; garden expects nanoseconds
		cpuStat = garden.ContainerCPUStat{
			Usage:  metrics.CPU.UsageUsec * 1000,
			User:   metrics.CPU.UserUsec * 1000,
			System: metrics.CPU.SystemUsec * 1000,
		}
	}

	var pidStat garden.ContainerPidStat
	if metrics.Pids != nil {
		pidStat = garden.ContainerPidStat{
			Current: metrics.Pids.Current,
			Max:     metrics.Pids.Limit,
		}
	}

	return gardener.StatsContainerMetrics{
		CPU:    cpuStat,
		Memory: memoryStat,
		Pid:    pidStat,
		Age:    time.Since(createdAt),
	}, nil
}

// UnmarshalContainerdMetrics unmarshals the protobuf Any from containerd task
// metrics response into cgroupsv2 Metrics.
func UnmarshalContainerdMetrics(data typeurl.Any) (*cgroup2stats.Metrics, error) {
	var m cgroup2stats.Metrics
	if err := typeurl.UnmarshalTo(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal cgroupsv2 metrics: %w", err)
	}
	return &m, nil
}

package runrunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . StatsNotifier
type StatsNotifier interface {
	OnStat(handle string, cpuStat garden.ContainerCPUStat, memoryStat garden.ContainerMemoryStat)
}

type runcStats struct {
	Data struct {
		CgroupStats struct {
			CPUStats struct {
				CPUUsage struct {
					Usage  uint64 `json:"total_usage"`
					System uint64 `json:"usage_in_kernelmode"`
					User   uint64 `json:"usage_in_usermode"`
				} `json:"cpu_usage"`
			} `json:"cpu_stats"`
			MemoryStats struct {
				Stats garden.ContainerMemoryStat `json:"stats"`
			} `json:"memory_stats"`
		} `json:"CgroupStats"`
	}
}

type Statser struct {
	runner RuncCmdRunner
	runc   RuncBinary
}

func NewStatser(runner RuncCmdRunner, runc RuncBinary) *Statser {
	return &Statser{
		runner, runc,
	}
}

func (r *Statser) Stats(log lager.Logger, id string) (gardener.ActualContainerMetrics, error) {
	buf := new(bytes.Buffer)

	if err := r.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		cmd := r.runc.StatsCommand(id, logFile)
		cmd.Stdout = buf
		return cmd
	}); err != nil {
		return gardener.ActualContainerMetrics{}, fmt.Errorf("runC stats: %s", err)
	}

	var data runcStats
	if err := json.NewDecoder(buf).Decode(&data); err != nil {
		return gardener.ActualContainerMetrics{}, fmt.Errorf("decode stats: %s", err)
	}

	stats := gardener.ActualContainerMetrics{
		Memory: data.Data.CgroupStats.MemoryStats.Stats,
		CPU: garden.ContainerCPUStat{
			Usage:  data.Data.CgroupStats.CPUStats.CPUUsage.Usage,
			System: data.Data.CgroupStats.CPUStats.CPUUsage.System,
			User:   data.Data.CgroupStats.CPUStats.CPUUsage.User,
		},
	}

	stats.Memory.TotalUsageTowardLimit = stats.Memory.TotalRss + (stats.Memory.TotalCache - stats.Memory.TotalInactiveFile)

	return stats, nil
}

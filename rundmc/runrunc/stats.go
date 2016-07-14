package runrunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . StatsNotifier
type StatsNotifier interface {
	OnStat(handle string, cpuStat garden.ContainerCPUStat, memoryStat garden.ContainerMemoryStat)
}

type runcStats struct {
	Data struct {
		CPUStats struct {
			CPUUsage struct {
				Usage  uint64 `json:"total"`
				System uint64 `json:"kernel"`
				User   uint64 `json:"user"`
			} `json:"usage"`
		} `json:"cpu"`
		MemoryStats struct {
			Stats garden.ContainerMemoryStat `json:"raw"`
		} `json:"memory"`
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
		Memory: data.Data.MemoryStats.Stats,
		CPU: garden.ContainerCPUStat{
			Usage:  data.Data.CPUStats.CPUUsage.Usage,
			System: data.Data.CPUStats.CPUUsage.System,
			User:   data.Data.CPUStats.CPUUsage.User,
		},
	}

	stats.Memory.TotalUsageTowardLimit = stats.Memory.TotalRss + (stats.Memory.TotalCache - stats.Memory.TotalInactiveFile)

	return stats, nil
}

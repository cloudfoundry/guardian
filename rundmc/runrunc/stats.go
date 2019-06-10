package runrunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

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
		PidStats struct {
			Current uint64 `json:"current"`
			Max     uint64 `json:"limit"`
		} `json:"pids"`
	}
}

type runcState struct {
	Created time.Time `json:"created"`
}

type Statser struct {
	runner RuncCmdRunner
	runc   RuncBinary
	depot  Depot
}

func NewStatser(runner RuncCmdRunner, runc RuncBinary, depot Depot) *Statser {
	return &Statser{
		runner, runc, depot,
	}
}

func (r *Statser) Stats(log lager.Logger, id string) (gardener.StatsContainerMetrics, error) {
	containerStats, err := r.getStats(log, id)
	if err != nil {
		return gardener.StatsContainerMetrics{}, err
	}

	containerState, err := r.getState(log, id)
	if err != nil {
		return gardener.StatsContainerMetrics{}, err
	}

	ctime := containerState.Created
	if ctime.IsZero() {
		var err error
		ctime, err = r.depot.CreatedTime(log, id)
		if err != nil {
			return gardener.StatsContainerMetrics{}, err
		}
	}

	stats := gardener.StatsContainerMetrics{
		Memory: containerStats.Data.MemoryStats.Stats,
		CPU: garden.ContainerCPUStat{
			Usage:  containerStats.Data.CPUStats.CPUUsage.Usage,
			System: containerStats.Data.CPUStats.CPUUsage.System,
			User:   containerStats.Data.CPUStats.CPUUsage.User,
		},
		Pid: garden.ContainerPidStat{
			Current: containerStats.Data.PidStats.Current,
			Max:     containerStats.Data.PidStats.Max,
		},
		Age: time.Since(ctime),
	}

	stats.Memory.TotalUsageTowardLimit = stats.Memory.TotalRss + (stats.Memory.TotalCache - stats.Memory.TotalInactiveFile)

	return stats, nil
}

func (r *Statser) getStats(log lager.Logger, id string) (runcStats, error) {
	buf := new(bytes.Buffer)

	if err := r.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		cmd := r.runc.StatsCommand(id, logFile)
		cmd.Stdout = buf
		return cmd
	}); err != nil {
		return runcStats{}, fmt.Errorf("runC stats: %s", err)
	}

	var data runcStats
	if err := json.NewDecoder(buf).Decode(&data); err != nil {
		return runcStats{}, fmt.Errorf("decode stats: %s", err)
	}

	return data, nil
}

func (r *Statser) getState(log lager.Logger, id string) (runcState, error) {
	stateBuf := new(bytes.Buffer)
	if err := r.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		cmd := r.runc.StateCommand(id, logFile)
		cmd.Stdout = stateBuf
		return cmd
	}); err != nil {
		return runcState{}, fmt.Errorf("runC state: %s", err)
	}
	var stateData runcState
	if err := json.NewDecoder(stateBuf).Decode(&stateData); err != nil {
		return runcState{}, fmt.Errorf("decode state: %s", err)
	}

	return stateData, nil
}

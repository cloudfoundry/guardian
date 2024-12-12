package cgroups

import (
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type CPUCgrouper struct {
	cgroupRoot string
}

func NewCPUCgrouper(cgroupRoot string) CPUCgrouper {
	return CPUCgrouper{
		cgroupRoot: cgroupRoot,
	}
}

func (c CPUCgrouper) PrepareCgroups(handle string) error {
	badCgroupPath := filepath.Join(c.cgroupRoot, BadCgroupName, handle)
	if err := os.MkdirAll(badCgroupPath, 0755); err != nil {
		return err
	}
	if err := enableSupportedControllers(badCgroupPath); err != nil {
		return err
	}
	return nil
}

func (c CPUCgrouper) CleanupCgroups(handle string) error {
	if err := os.RemoveAll(filepath.Join(c.cgroupRoot, BadCgroupName, handle)); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(c.cgroupRoot, GoodCgroupName, handle)); err != nil {
		return err
	}
	return nil
}

func (c CPUCgrouper) ReadBadCgroupUsage(handle string) (garden.ContainerCPUStat, error) {
	path := filepath.Join(c.cgroupRoot, BadCgroupName, handle)

	var stats *cgroups.Stats

	if cgroups.IsCgroup2UnifiedMode() {
		cgroupManager, err := fs2.NewManager(&configs.Cgroup{}, path)
		if err != nil {
			return garden.ContainerCPUStat{}, err
		}
		stats, err = cgroupManager.GetStats()
		if err != nil {
			return garden.ContainerCPUStat{}, err
		}
	} else {
		cpuactCgroup := &fs.CpuacctGroup{}

		if _, err := os.Stat(path); err != nil {
			return garden.ContainerCPUStat{}, err
		}

		if err := cpuactCgroup.GetStats(path, stats); err != nil {
			return garden.ContainerCPUStat{}, err
		}
	}

	if stats != nil {
		cpuStats := garden.ContainerCPUStat{
			Usage:  stats.CpuStats.CpuUsage.TotalUsage,
			System: stats.CpuStats.CpuUsage.UsageInKernelmode,
			User:   stats.CpuStats.CpuUsage.UsageInUsermode,
		}

		return cpuStats, nil
	}

	return garden.ContainerCPUStat{}, fmt.Errorf("failed-to-read-bad-cgroup-stats")
}

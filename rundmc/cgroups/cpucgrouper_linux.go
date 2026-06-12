// @AI-Generated
// Generated in whole or in part by Cursor with a mix of different LLM models (Auto select mode)
// Description:
// 2026-06-11: Add PropagateContainerMemoryLimit for cgroupsv2 PEA memory limit inheritance (CPU throttling path)
// 2026-06-11: Also propagate memory.swap.max=0 so PEAs cannot bypass the limit via swap

package cgroups

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs"
	"github.com/opencontainers/cgroups/fs2"
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
	if cgroups.IsCgroup2UnifiedMode() {
		if err := EnableSupportedControllers(badCgroupPath); err != nil {
			return err
		}
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

// PropagateContainerMemoryLimit writes the container memory limit to the
// good-cgroup sandbox path so PEAs (siblings of the container's init
// sub-cgroup) inherit the same memory constraint on cgroupsv2.
// When disableSwapLimit is false, memory.swap.max is also set to 0 so
// that PEAs cannot bypass the memory limit by swapping.
func (c CPUCgrouper) PropagateContainerMemoryLimit(handle string, memoryLimit int64, disableSwapLimit bool) error {
	if !cgroups.IsCgroup2UnifiedMode() || memoryLimit <= 0 {
		return nil
	}
	sandboxCgroupPath := filepath.Join(c.cgroupRoot, GoodCgroupName, handle)
	if err := os.WriteFile(
		filepath.Join(sandboxCgroupPath, "memory.max"),
		[]byte(fmt.Sprintf("%d", memoryLimit)),
		0644,
	); err != nil {
		return err
	}
	if !disableSwapLimit {
		err := os.WriteFile(filepath.Join(sandboxCgroupPath, "memory.swap.max"), []byte("0"), 0644)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (c CPUCgrouper) ReadTotalCgroupUsage(handle string, containerCPUStats garden.ContainerCPUStat) (garden.ContainerCPUStat, error) {
	badPath := filepath.Join(c.cgroupRoot, BadCgroupName, handle)
	badCPUStats, err := readCPUstatsFromPath(badPath)
	if err != nil {
		return garden.ContainerCPUStat{}, err
	}

	goodPath := filepath.Join(c.cgroupRoot, GoodCgroupName, handle)
	goodCPUStats, err := readCPUstatsFromPath(goodPath)
	if err != nil {
		return garden.ContainerCPUStat{}, err
	}

	cpuStats := garden.ContainerCPUStat{
		Usage:  badCPUStats.CpuStats.CpuUsage.TotalUsage + goodCPUStats.CpuStats.CpuUsage.TotalUsage,
		System: badCPUStats.CpuStats.CpuUsage.UsageInKernelmode + goodCPUStats.CpuStats.CpuUsage.UsageInKernelmode,
		User:   badCPUStats.CpuStats.CpuUsage.UsageInUsermode + goodCPUStats.CpuStats.CpuUsage.UsageInUsermode,
	}

	return cpuStats, nil
}

func readCPUstatsFromPath(path string) (cgroups.Stats, error) {
	stats := &cgroups.Stats{}

	if cgroups.IsCgroup2UnifiedMode() {
		cgroupManager, err := fs2.NewManager(&cgroups.Cgroup{}, path)
		if err != nil {
			return cgroups.Stats{}, err
		}
		stats, err = cgroupManager.GetStats()
		if err != nil {
			return cgroups.Stats{}, err
		}
	} else {
		cpuactCgroup := &fs.CpuacctGroup{}

		if _, err := os.Stat(path); err != nil {
			return cgroups.Stats{}, err
		}

		if err := cpuactCgroup.GetStats(path, stats); err != nil {
			return cgroups.Stats{}, err
		}
	}

	return *stats, nil
}

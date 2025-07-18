package throttle

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/guardian/rundmc"
	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/lager/v3"
	"github.com/opencontainers/cgroups"
)

const (
	MB           uint64 = 1024 * 1024
	MaxCPUWeight uint64 = 10000
)

type SharesBalancer struct {
	memoryProvider MemoryProvider
	goodCgroupPath string
	badCgroupPath  string
	multiplier     float64
}

func NewSharesBalancer(cpuCgroupPath string, memoryProvider MemoryProvider, multiplier float64) SharesBalancer {
	return SharesBalancer{
		memoryProvider: memoryProvider,
		goodCgroupPath: filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName),
		badCgroupPath:  filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName),
		multiplier:     multiplier,
	}
}

func (b SharesBalancer) Run(logger lager.Logger) error {
	logger = logger.Session("sharebalancer")
	logger.Info("starting")
	defer logger.Info("finished")

	totalMemoryInBytes, _ := b.memoryProvider.TotalMemory()
	fmt.Println("totalMemoryInBytes ****", totalMemoryInBytes)

	badShares, err := b.countShares(b.badCgroupPath)
	if err != nil {
		return err
	}

	badShares = uint64(float64(badShares) * b.multiplier)

	if badShares == 0 {
		badShares = 2
	}
	goodShares := totalMemoryInBytes/MB - badShares

	err = b.setShares(logger, b.goodCgroupPath, goodShares)
	if err != nil {
		return err
	}

	err = b.setShares(logger, b.badCgroupPath, badShares)
	if err != nil {
		return err
	}

	return nil
}

func (b SharesBalancer) countShares(cgroupPath string) (uint64, error) {
	children, err := os.ReadDir(cgroupPath)
	if err != nil {
		return 0, err
	}

	var totalShares uint64
	for _, child := range children {
		fmt.Println("reading child *** ", child)
		if !child.IsDir() {
			continue
		}

		childPath := filepath.Join(cgroupPath, child.Name())

		if !hasProcs(childPath) {
			continue
		}

		shares, err := b.getShares(childPath)
		if err != nil {
			return 0, err
		}

		totalShares += shares
		fmt.Println("totalShares  :", totalShares)
	}

	return totalShares, nil
}

func (b SharesBalancer) getShares(cgroupPath string) (uint64, error) {
	if cgroups.IsCgroup2UnifiedMode() {
		bytes, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.weight"))
		if err != nil {
			return 0, err
		}

		weight, err := strconv.ParseUint(strings.TrimSpace(string(bytes)), 10, 64)
		if err != nil {
			return 0, err
		}
		return rundmc.ConvertCgroupV2ValueToCPUShares(weight), nil
	}

	bytes, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.shares"))
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(strings.TrimSpace(string(bytes)), 10, 64)
}

func (b SharesBalancer) setShares(logger lager.Logger, cgroupPath string, shares uint64) error {
	logger.Info("set-shares", lager.Data{"cgroupPath": cgroupPath, "shares": shares})
	if cgroups.IsCgroup2UnifiedMode() {
		weight := cgroups.ConvertCPUSharesToCgroupV2Value(shares)
		// When sum of bad shares exceed total memory we get a negative number which translates to large number
		// For cpu.shares in cgroups v1 this gets automatically set to MAX_SHARES
		// This is questionable behavior for cgroups v1 but at this point we just mimic this behavior
		if weight > MaxCPUWeight {
			weight = MaxCPUWeight
		}
		return os.WriteFile(filepath.Join(cgroupPath, "cpu.weight"), []byte(strconv.FormatUint(weight, 10)), 0644)
	}
	return os.WriteFile(filepath.Join(cgroupPath, "cpu.shares"), []byte(strconv.FormatUint(shares, 10)), 0644)
}

func hasProcs(cgroupPath string) bool {
	pids, err := cgroups.GetAllPids(cgroupPath)
	if err != nil {
		return false
	}

	return len(pids) != 0
}

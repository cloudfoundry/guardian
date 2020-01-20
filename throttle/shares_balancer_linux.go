package throttle

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/lager"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const MB uint64 = 1024 * 1024

type SharesBalancer struct {
	memoryProvider MemoryProvider
	goodCgroupPath string
	badCgroupPath  string
}

//go:generate counterfeiter . MemoryProvider
type MemoryProvider interface {
	TotalMemory() (uint64, error)
}

func NewSharesBalancer(cpuCgroupPath string, memoryProvider MemoryProvider) SharesBalancer {
	return SharesBalancer{
		memoryProvider: memoryProvider,
		goodCgroupPath: filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName),
		badCgroupPath:  filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName),
	}
}

func (b SharesBalancer) Run(logger lager.Logger) error {
	logger = logger.Session("sharebalancer")
	logger.Info("starting")
	defer logger.Info("finished")

	totalMemoryInBytes, _ := b.memoryProvider.TotalMemory()

	badShares, err := countShares(b.badCgroupPath)
	if err != nil {
		return err
	}

	if badShares == 0 {
		badShares = 2
	}
	goodShares := totalMemoryInBytes/MB - badShares

	err = setShares(logger, b.goodCgroupPath, goodShares)
	if err != nil {
		return err
	}

	err = setShares(logger, b.badCgroupPath, badShares)
	if err != nil {
		return err
	}

	return nil
}

func countShares(cgroupPath string) (uint64, error) {
	children, err := ioutil.ReadDir(cgroupPath)
	if err != nil {
		return 0, err
	}

	var totalShares uint64
	for _, child := range children {
		if !child.IsDir() {
			continue
		}

		childPath := filepath.Join(cgroupPath, child.Name())

		if !hasProcs(childPath) {
			continue
		}

		shares, err := getShares(childPath)
		if err != nil {
			return 0, err
		}

		totalShares += shares
	}

	return totalShares, nil
}

func getShares(cgroupPath string) (uint64, error) {
	bytes, err := ioutil.ReadFile(filepath.Join(cgroupPath, "cpu.shares"))
	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(strings.TrimSpace(string(bytes)), 10, 64)
}

func setShares(logger lager.Logger, cgroupPath string, shares uint64) error {
	logger.Info("set-shares", lager.Data{"cgroupPath": cgroupPath, "shares": shares})
	return ioutil.WriteFile(filepath.Join(cgroupPath, "cpu.shares"), []byte(strconv.FormatUint(shares, 10)), 0644)
}

func hasProcs(cgroupPath string) bool {
	pids, err := cgroups.GetAllPids(cgroupPath)
	if err != nil {
		return false
	}

	return len(pids) != 0
}

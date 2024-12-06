package throttle

import (
	"os"
	"path/filepath"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"

	"code.cloudfoundry.org/lager/v3"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

type CPUCgroupEnforcer struct {
	goodCgroupPath string
	badCgroupPath  string
	cpuSharesFile  string
}

func NewEnforcer(cpuCgroupPath string) CPUCgroupEnforcer {
	cpuSharesFile := "cpu.shares"
	if cgroups.IsCgroup2UnifiedMode() {
		cpuSharesFile = "cpu.weight"
	}

	return CPUCgroupEnforcer{
		goodCgroupPath: filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName),
		badCgroupPath:  filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName),
		cpuSharesFile:  cpuSharesFile,
	}
}

func (c CPUCgroupEnforcer) Punish(logger lager.Logger, handle string) error {
	logger = logger.Session("punish", lager.Data{"handle": handle})
	logger.Info("starting")
	defer logger.Info("finished")

	goodContainerCgroupPath := filepath.Join(c.goodCgroupPath, handle)
	if !exists(logger, goodContainerCgroupPath) {
		logger.Info("good-cgroup-does-not-exist-skip-punish", lager.Data{"handle": handle, "goodContainerCgroupPath": goodContainerCgroupPath})
		return nil
	}

	badContainerCgroupPath := filepath.Join(c.badCgroupPath, handle)

	// for peas in cgroups v2 processes are added to init cgroup
	goodInitCgroupPath := filepath.Join(goodContainerCgroupPath, gardencgroups.InitCgroup)
	if exists(logger, goodInitCgroupPath) {
		if err := c.movePids(goodInitCgroupPath, badContainerCgroupPath); err != nil {
			return err
		}

		return c.copyShares(goodInitCgroupPath, badContainerCgroupPath)
	}

	if err := c.movePids(goodContainerCgroupPath, badContainerCgroupPath); err != nil {
		return err
	}

	return c.copyShares(goodContainerCgroupPath, badContainerCgroupPath)
}

func (c CPUCgroupEnforcer) Release(logger lager.Logger, handle string) error {
	logger = logger.Session("release", lager.Data{"handle": handle})
	logger.Info("starting")
	defer logger.Info("finished")

	badContainerCgroupPath := filepath.Join(c.badCgroupPath, handle)
	if !exists(logger, badContainerCgroupPath) {
		logger.Info("bad-cgroup-does-not-exist-skip-punish", lager.Data{"handle": handle, "badContainerCgroupPath": badContainerCgroupPath})
		return nil
	}

	goodContainerCgroupPath := filepath.Join(c.goodCgroupPath, handle)

	// for peas in cgroups v2 processes are added to init cgroup
	goodInitCgroupPath := filepath.Join(goodContainerCgroupPath, gardencgroups.InitCgroup)
	if exists(logger, goodInitCgroupPath) {
		return c.movePids(badContainerCgroupPath, goodInitCgroupPath)
	}

	return c.movePids(badContainerCgroupPath, goodContainerCgroupPath)
}

func (c CPUCgroupEnforcer) movePids(fromCgroup, toCgroup string) error {
	for {
		pids, err := cgroups.GetPids(fromCgroup)
		if err != nil {
			return err
		}

		if len(pids) == 0 {
			return nil
		}

		for _, pid := range pids {
			if err = cgroups.WriteCgroupProc(toCgroup, pid); err != nil {
				return err
			}
		}
	}
}

func (c CPUCgroupEnforcer) copyShares(fromCgroup, toCgroup string) error {
	containerShares, err := os.ReadFile(filepath.Join(fromCgroup, c.cpuSharesFile))
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(toCgroup, c.cpuSharesFile), containerShares, 0644)
}

func exists(logger lager.Logger, cgroupPath string) bool {
	_, err := os.Stat(cgroupPath)
	if err == nil {
		return true
	}

	if !os.IsNotExist(err) {
		logger.Error("failed-to-stat-cgroup-path", err, lager.Data{"cgroupPath": cgroupPath})
	}

	return false
}

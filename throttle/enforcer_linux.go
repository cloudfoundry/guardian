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
}

func NewEnforcer(cpuCgroupPath string) CPUCgroupEnforcer {
	return CPUCgroupEnforcer{
		goodCgroupPath: filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName),
		badCgroupPath:  filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName),
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

	if err := movePids(goodContainerCgroupPath, badContainerCgroupPath); err != nil {
		return err
	}

	return copyShares(goodContainerCgroupPath, badContainerCgroupPath)
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

	return movePids(badContainerCgroupPath, goodContainerCgroupPath)
}

func movePids(fromCgroup, toCgroup string) error {
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

func copyShares(fromCgroup, toCgroup string) error {
	containerShares, err := os.ReadFile(filepath.Join(fromCgroup, "cpu.shares"))
	if err != nil {
		return err
	}

	return writeCPUShares(toCgroup, containerShares)
}

func writeCPUShares(cgroupPath string, shares []byte) error {
	return os.WriteFile(filepath.Join(cgroupPath, "cpu.shares"), shares, 0644)
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

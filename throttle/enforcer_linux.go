package throttle

import (
	"encoding/json"
	"os"
	"path/filepath"

	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"

	"code.cloudfoundry.org/lager/v3"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/utils"
)

type CPUCgroupEnforcer struct {
	goodCgroupPath string
	badCgroupPath  string
	cpuSharesFile  string
	runcRoot       string
	namespace      string
}

func NewEnforcer(cpuCgroupPath string, runcRoot string, namespace string) CPUCgroupEnforcer {
	cpuSharesFile := "cpu.shares"
	if cgroups.IsCgroup2UnifiedMode() {
		cpuSharesFile = "cpu.weight"
	}

	return CPUCgroupEnforcer{
		goodCgroupPath: filepath.Join(cpuCgroupPath, gardencgroups.GoodCgroupName),
		badCgroupPath:  filepath.Join(cpuCgroupPath, gardencgroups.BadCgroupName),
		cpuSharesFile:  cpuSharesFile,
		runcRoot:       runcRoot,
		namespace:      namespace,
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
		if err := c.copyShares(goodInitCgroupPath, badContainerCgroupPath); err != nil {
			return err
		}

		if err := c.movePids(goodInitCgroupPath, badContainerCgroupPath); err != nil {
			return err
		}

		return c.updateContainerStateCgroupPath(handle, badContainerCgroupPath)
	}

	if err := c.copyShares(goodContainerCgroupPath, badContainerCgroupPath); err != nil {
		return err
	}

	if err := c.movePids(goodContainerCgroupPath, badContainerCgroupPath); err != nil {
		return err
	}

	return c.updateContainerStateCgroupPath(handle, badContainerCgroupPath)
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
		if err := c.movePids(badContainerCgroupPath, goodInitCgroupPath); err != nil {
			return err
		}
		return c.updateContainerStateCgroupPath(handle, goodInitCgroupPath)
	}

	if err := c.movePids(badContainerCgroupPath, goodContainerCgroupPath); err != nil {
		return err
	}
	return c.updateContainerStateCgroupPath(handle, goodContainerCgroupPath)
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

// Runc pulls container cgroup path from the container state file
// In cgroup v1, runc is using cgroup path for device to determine container pid files
// In cgroup v2, runc is using unified cgroup path which needs to be updated
func (c CPUCgroupEnforcer) updateContainerStateCgroupPath(handle string, cgroupPath string) (retErr error) {
	if !cgroups.IsCgroup2UnifiedMode() {
		return nil
	}

	stateDir := filepath.Join(c.runcRoot, c.namespace)
	statePath := filepath.Join(stateDir, handle, "state.json")
	stateFile, err := os.Open(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer stateFile.Close()

	var state libcontainer.State
	err = json.NewDecoder(stateFile).Decode(&state)
	if err != nil {
		return err
	}

	state.CgroupPaths[""] = cgroupPath

	tmpFile, err := os.CreateTemp(stateDir, "state-")
	if err != nil {
		return err
	}

	defer func() {
		if retErr != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
		}
	}()

	err = utils.WriteJSON(tmpFile, state)
	if err != nil {
		return err
	}
	err = tmpFile.Close()
	if err != nil {
		return err
	}

	return os.Rename(tmpFile.Name(), statePath)
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

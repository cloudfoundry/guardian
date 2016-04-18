package stopper

import (
	"syscall"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/pivotal-golang/lager"
)

func (stopper *CgroupStopper) StopAll(log lager.Logger, cgroupName string, exceptions []int, kill bool) error {
	log = log.Session("stop-all", lager.Data{
		"name": cgroupName,
	})

	log.Debug("start")
	defer log.Debug("finished")

	devicesSubsystemPath, err := stopper.cgroupPathResolver.Resolve(cgroupName, "devices")
	if err != nil {
		return err
	}

	pidsInCgroup, err := cgroups.GetAllPids(devicesSubsystemPath)
	if err != nil {
		return err
	}

	var pidsToKill []int

PIDS:
	for _, pid := range pidsInCgroup {
		for _, p := range exceptions {
			if pid == p {
				continue PIDS
			}
		}

		pidsToKill = append(pidsToKill, pid)
	}

	stopper.killer.Kill(syscall.SIGTERM, pidsToKill...)
	return nil
}

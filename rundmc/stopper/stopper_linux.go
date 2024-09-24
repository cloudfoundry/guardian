package stopper

import (
	"fmt"
	"syscall"

	"code.cloudfoundry.org/lager/v3"
	"github.com/opencontainers/runc/libcontainer/cgroups"
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

	if !kill {
		err := stopper.retrier.Run(func() error {
			return stopper.killAllRemaining(syscall.SIGTERM, devicesSubsystemPath, exceptions)
		})
		if err != nil {
			log.Debug("failed-to-kill-remaining-processes", lager.Data{"error": err, "name": cgroupName})
		}
	}

	err = stopper.retrier.Run(func() error {
		return stopper.killAllRemaining(syscall.SIGKILL, devicesSubsystemPath, exceptions)
	})
	if err != nil {
		log.Debug("failed-to-kill-remaining-processes", lager.Data{"error": err, "name": cgroupName})
	}

	return nil // we killed, so everything must die
}

func (stopper *CgroupStopper) killAllRemaining(signal syscall.Signal, cgroupPath string, exceptions []int) error {
	pidsInCgroup, err := cgroups.GetAllPids(cgroupPath)
	if err != nil {
		return err
	}

	var pidsToKill []int
	for _, pid := range pidsInCgroup {
		if contains(exceptions, pid) {
			continue
		}

		pidsToKill = append(pidsToKill, pid)
	}

	if len(pidsToKill) == 0 {
		return nil
	}

	stopper.killer.Kill(signal, pidsToKill...)
	return fmt.Errorf("still running after signal %s, %v", signal, pidsToKill)
}

func contains(a []int, b int) bool {
	for _, i := range a {
		if i == b {
			return true
		}
	}

	return false
}

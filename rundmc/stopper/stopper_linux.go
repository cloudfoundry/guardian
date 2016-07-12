package stopper

import (
	"fmt"
	"syscall"

	"code.cloudfoundry.org/lager"
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
		stopper.retrier.Run(func() error {
			return stopper.killAllRemaining(syscall.SIGTERM, devicesSubsystemPath, exceptions)
		})
	}

	stopper.retrier.Run(func() error {
		return stopper.killAllRemaining(syscall.SIGKILL, devicesSubsystemPath, exceptions)
	})

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

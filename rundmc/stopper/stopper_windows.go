package stopper

import (
	"syscall"

	"code.cloudfoundry.org/lager"
)

func (stopper *CgroupStopper) StopAll(log lager.Logger, cgroupName string, exceptions []int, kill bool) error {
	return nil
}

func (stopper *CgroupStopper) killAllRemaining(signal syscall.Signal, cgroupPath string, exceptions []int) error {
	return nil
}

package stopper

import (
	"code.cloudfoundry.org/lager/v3"
)

func (stopper *CgroupStopper) StopAll(log lager.Logger, cgroupName string, exceptions []int, kill bool) error {
	return nil
}

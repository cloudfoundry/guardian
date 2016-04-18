package stopper

import "syscall"

//go:generate counterfeiter . Killer
//go:generate counterfeiter . CgroupPathResolver

type Killer interface {
	Kill(signal syscall.Signal, pid ...int)
}

type CgroupPathResolver interface {
	Resolve(cgroupName, subsystem string) (string, error)
}

type CgroupStopper struct {
	killer             Killer
	cgroupPathResolver CgroupPathResolver
}

func New(cgroupPathResolver CgroupPathResolver, killer Killer) *CgroupStopper {
	if killer == nil {
		killer = DefaultKiller{}
	}

	return &CgroupStopper{
		killer:             killer,
		cgroupPathResolver: cgroupPathResolver,
	}
}

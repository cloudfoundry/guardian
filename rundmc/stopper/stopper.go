package stopper

import "syscall"

//counterfeiter:generate . Killer
//counterfeiter:generate . CgroupPathResolver
//counterfeiter:generate . Retrier

type Killer interface {
	Kill(signal syscall.Signal, pid ...int)
}

type CgroupPathResolver interface {
	Resolve(cgroupName, subsystem string) (string, error)
}

type Retrier interface {
	Run(work func() error) error
}

type CgroupStopper struct {
	killer             Killer
	retrier            Retrier
	cgroupPathResolver CgroupPathResolver
}

func New(cgroupPathResolver CgroupPathResolver, killer Killer, retrier Retrier) *CgroupStopper {
	if killer == nil {
		killer = DefaultKiller{}
	}

	return &CgroupStopper{
		killer:             killer,
		cgroupPathResolver: cgroupPathResolver,
		retrier:            retrier,
	}
}

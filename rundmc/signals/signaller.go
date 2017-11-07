package signals

import (
	"fmt"
	"os"
	"syscall"

	"code.cloudfoundry.org/garden"
)

//go:generate counterfeiter . Signaller
type Signaller interface {
	Signal(signal garden.Signal) error
}

type signaller struct {
	pidFilePath string
	pidGetter   PidGetter
}

//go:generate counterfeiter . PidGetter
type PidGetter interface {
	Pid(pidFilePath string) (int, error)
}

type SignallerFactory struct {
	PidGetter PidGetter
}

func (f *SignallerFactory) NewSignaller(pidfilePath string) Signaller {
	return &signaller{pidFilePath: pidfilePath, pidGetter: f.PidGetter}
}

func (s *signaller) Signal(signal garden.Signal) error {
	pid, err := s.pidGetter.Pid(s.pidFilePath)
	if err != nil {
		return fmt.Errorf("fetching-pid: %s", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		// Should never happen, os.FindProcess never returns an error on Linux
		return fmt.Errorf("finding-process: %s", err)
	}

	return process.Signal(osSignal(signal).OsSignal())
}

type osSignal garden.Signal

func (s osSignal) OsSignal() syscall.Signal {
	switch garden.Signal(s) {
	case garden.SignalTerminate:
		return syscall.SIGTERM
	default:
		return syscall.SIGKILL
	}
}

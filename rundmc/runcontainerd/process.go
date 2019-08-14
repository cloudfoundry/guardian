package runcontainerd

import (
	"fmt"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . BackingProcess

type BackingProcess interface {
	ID() string
	Signal(syscall.Signal) error
	Wait() (int, error)
	Delete() error
}

type Process struct {
	log                      lager.Logger
	nerdProcess              BackingProcess
	cleanupProcessDirsOnWait bool
}

func NewProcess(log lager.Logger, nerdProcess BackingProcess, cleanupProcessDirsOnWait bool) *Process {
	return &Process{
		log:                      log,
		nerdProcess:              nerdProcess,
		cleanupProcessDirsOnWait: cleanupProcessDirsOnWait,
	}
}

func (p *Process) ID() string {
	return p.nerdProcess.ID()
}

func (p *Process) Wait() (int, error) {
	exitStatus, err := p.nerdProcess.Wait()
	if err != nil {
		return 0, err
	}

	if p.cleanupProcessDirsOnWait {
		p.log.Debug("wait.cleanup-process", lager.Data{"processID": p.nerdProcess.ID()})
		err = p.nerdProcess.Delete()
		if err != nil {
			p.log.Error("cleanup-failed-deleting-process", err)
		}
	}

	return exitStatus, nil
}

func (p *Process) Signal(gardenSignal garden.Signal) error {
	signal, err := toSyscallSignal(gardenSignal)
	if err != nil {
		return err
	}

	return p.nerdProcess.Signal(signal)
}

func (p *Process) SetTTY(garden.TTYSpec) error {
	return nil
}

func toSyscallSignal(signal garden.Signal) (syscall.Signal, error) {
	switch signal {
	case garden.SignalTerminate:
		return syscall.SIGTERM, nil
	case garden.SignalKill:
		return syscall.SIGKILL, nil
	}

	return -1, fmt.Errorf("Cannot convert garden signal %d to syscall.Signal", signal)
}

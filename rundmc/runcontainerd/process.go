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
}

type Process struct {
	log     lager.Logger
	process BackingProcess
}

func NewProcess(log lager.Logger, backingProcess BackingProcess) *Process {
	return &Process{log: log, process: backingProcess}
}

func (p *Process) ID() string {
	return p.process.ID()
}

func (p *Process) Wait() (int, error) {
	return p.process.Wait()
}

func (p *Process) Signal(gardenSignal garden.Signal) error {
	signal, err := toSyscallSignal(gardenSignal)
	if err != nil {
		return err
	}

	return p.process.Signal(signal)
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

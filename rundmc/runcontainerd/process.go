package runcontainerd

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

type Process struct {
	log         lager.Logger
	containerID string
	processID   string

	processManager ProcessManager
}

func NewProcess(log lager.Logger, containerID, processID string, processManager ProcessManager) *Process {
	return &Process{log: log, containerID: containerID, processID: processID, processManager: processManager}
}

func (p *Process) ID() string { return "" }

func (p *Process) Wait() (int, error)          { return p.processManager.Wait(p.log, p.containerID, p.processID) }
func (p *Process) SetTTY(garden.TTYSpec) error { return nil }
func (p *Process) Signal(garden.Signal) error  { return nil }

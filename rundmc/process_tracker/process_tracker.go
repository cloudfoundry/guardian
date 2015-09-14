package process_tracker

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry/gunk/command_runner"
)

//go:generate counterfeiter -o fake_process_tracker/fake_process_tracker.go . ProcessTracker
type ProcessTracker interface {
	Run(processID uint32, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec, signaller Signaller) (garden.Process, error)
	Attach(processID uint32, io garden.ProcessIO) (garden.Process, error)
	Restore(processID uint32, signaller Signaller)
	ActiveProcesses() []garden.Process
}

type processTracker struct {
	containerPath string
	runner        command_runner.CommandRunner

	iodaemonBin string

	processes      map[uint32]*Process
	processesMutex *sync.RWMutex
}

type UnknownProcessError struct {
	ProcessID uint32
}

func (e UnknownProcessError) Error() string {
	return fmt.Sprintf("process_tracker: unknown process: %d", e.ProcessID)
}

func New(containerPath string, iodaemonBin string, runner command_runner.CommandRunner) ProcessTracker {
	return &processTracker{
		containerPath: containerPath,
		runner:        runner,

		iodaemonBin: iodaemonBin,

		processesMutex: new(sync.RWMutex),
		processes:      make(map[uint32]*Process),
	}
}

func (t *processTracker) Run(processID uint32, cmd *exec.Cmd, processIO garden.ProcessIO, tty *garden.TTYSpec, signaller Signaller) (garden.Process, error) {
	t.processesMutex.Lock()
	process := NewProcess(processID, t.containerPath, t.iodaemonBin, t.runner, signaller)
	t.processes[processID] = process
	t.processesMutex.Unlock()

	ready, active := process.Spawn(cmd, tty)

	err := <-ready
	if err != nil {
		return nil, err
	}

	process.Attach(processIO)

	go t.link(process.ID())

	err = <-active
	if err != nil {
		return nil, err
	}

	return process, nil
}

func (t *processTracker) Attach(processID uint32, processIO garden.ProcessIO) (garden.Process, error) {
	t.processesMutex.RLock()
	process, ok := t.processes[processID]
	t.processesMutex.RUnlock()

	if !ok {
		return nil, UnknownProcessError{processID}
	}

	process.Attach(processIO)

	go t.link(processID)

	return process, nil
}

func (t *processTracker) Restore(processID uint32, signaller Signaller) {
	t.processesMutex.Lock()

	process := NewProcess(processID, t.containerPath, t.iodaemonBin, t.runner, signaller)

	t.processes[processID] = process

	go t.link(processID)

	t.processesMutex.Unlock()
}

func (t *processTracker) ActiveProcesses() []garden.Process {
	t.processesMutex.RLock()
	defer t.processesMutex.RUnlock()

	processes := make([]garden.Process, len(t.processes))

	i := 0
	for _, process := range t.processes {
		processes[i] = process
		i++
	}

	return processes
}

func (t *processTracker) link(processID uint32) {
	t.processesMutex.RLock()
	process, ok := t.processes[processID]
	t.processesMutex.RUnlock()

	if !ok {
		return
	}

	defer t.unregister(processID)

	process.Link()

	return
}

func (t *processTracker) unregister(processID uint32) {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	delete(t.processes, processID)
}

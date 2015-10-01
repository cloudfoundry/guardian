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
	Run(processID string, cmd *exec.Cmd, io garden.ProcessIO, tty *garden.TTYSpec, signaller Signaller) (garden.Process, error)
	Attach(processID string, io garden.ProcessIO) (garden.Process, error)
	Restore(processID string, signaller Signaller)
	ActiveProcesses() []garden.Process
}

type processTracker struct {
	containerPath string
	runner        command_runner.CommandRunner

	iodaemonBin string

	processes      map[string]*Process
	processesMutex *sync.RWMutex
}

type UnknownProcessError struct {
	ProcessID string
}

func (e UnknownProcessError) Error() string {
	return fmt.Sprintf("process_tracker: unknown process: %s", e.ProcessID)
}

func New(containerPath string, iodaemonBin string, runner command_runner.CommandRunner) ProcessTracker {
	return &processTracker{
		containerPath: containerPath,
		runner:        runner,

		iodaemonBin: iodaemonBin,

		processesMutex: new(sync.RWMutex),
		processes:      make(map[string]*Process),
	}
}

func (t *processTracker) Run(processID string, cmd *exec.Cmd, processIO garden.ProcessIO, tty *garden.TTYSpec, signaller Signaller) (garden.Process, error) {
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

func (t *processTracker) Attach(processID string, processIO garden.ProcessIO) (garden.Process, error) {
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

func (t *processTracker) Restore(processID string, signaller Signaller) {
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

func (t *processTracker) link(processID string) {
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

func (t *processTracker) unregister(processID string) {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	delete(t.processes, processID)
}

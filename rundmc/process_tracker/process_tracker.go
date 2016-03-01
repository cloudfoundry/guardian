package process_tracker

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry/gunk/command_runner"
)

//go:generate counterfeiter . PidGetter
type PidGetter interface {
	Pid(pidFilePath string) (int, error)
}

type ProcessTracker struct {
	containerPath string
	iodaemonBin   string
	runner        command_runner.CommandRunner
	pidGetter     PidGetter

	processes      map[string]*Process
	processesMutex *sync.RWMutex
}

type UnknownProcessError struct {
	ProcessID string
}

func (e UnknownProcessError) Error() string {
	return fmt.Sprintf("process_tracker: unknown process: %s", e.ProcessID)
}

func New(
	containerPath string,
	iodaemonBin string,
	runner command_runner.CommandRunner,
	pidGetter PidGetter,
) *ProcessTracker {
	return &ProcessTracker{
		containerPath: containerPath,
		iodaemonBin:   iodaemonBin,
		runner:        runner,
		pidGetter:     pidGetter,

		processesMutex: new(sync.RWMutex),
		processes:      make(map[string]*Process),
	}
}

func (t *ProcessTracker) Run(processID string, cmd *exec.Cmd, processIO garden.ProcessIO, tty *garden.TTYSpec, pidFilePath string) (garden.Process, error) {
	t.processesMutex.Lock()
	process := NewProcess(t.containerPath, t.iodaemonBin, t.runner, t.pidGetter, processID, pidFilePath)
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

func (t *ProcessTracker) Attach(processID string, processIO garden.ProcessIO) (garden.Process, error) {
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

func (t *ProcessTracker) Restore(processID string) {
	t.processesMutex.Lock()

	process := NewProcess(t.containerPath, t.iodaemonBin, t.runner, t.pidGetter, processID, "")

	t.processes[processID] = process

	go t.link(processID)

	t.processesMutex.Unlock()
}

func (t *ProcessTracker) ActiveProcesses() []garden.Process {
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

func (t *ProcessTracker) link(processID string) {
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

func (t *ProcessTracker) unregister(processID string) {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	delete(t.processes, processID)
}

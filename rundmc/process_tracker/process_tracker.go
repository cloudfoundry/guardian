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
	process := t.getOrCreateProcess(processID, pidFilePath)
	ready, active := process.Spawn(cmd, tty)

	err := <-ready
	if err != nil {
		return nil, err
	}

	process.Attach(processIO)

	go process.Link(t.unregister)

	err = <-active
	if err != nil {
		return nil, err
	}

	return process, nil
}

func (t *ProcessTracker) Attach(processID string, processIO garden.ProcessIO, pidFilePath string) (garden.Process, error) {
	process := t.getOrCreateProcess(processID, pidFilePath)
	go process.Link(t.unregister)

	process.Attach(processIO)

	return process, nil
}

func (t *ProcessTracker) getOrCreateProcess(processID string, pidFilePath string) *Process {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	process, ok := t.processes[processID]
	if !ok {
		process = NewProcess(t.containerPath, t.iodaemonBin, t.runner, t.pidGetter, processID, pidFilePath)
		t.processes[processID] = process
	}

	return process
}

func (t *ProcessTracker) unregister(processID string) {
	t.processesMutex.Lock()
	defer t.processesMutex.Unlock()

	delete(t.processes, processID)
}

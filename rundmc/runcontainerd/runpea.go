package runcontainerd

import (
	"io"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Creator
type Creator interface {
	Create(log lager.Logger, bundlePath, id string, io garden.ProcessIO) error
}

type RunContainerPea struct {
	Creator        Creator
	ProcessManager ProcessManager
}

func (r *RunContainerPea) Run(
	log lager.Logger, processID, processPath, sandboxHandle, sandboxBundlePath string,
	pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
) (garden.Process, error) {

	if err := r.Creator.Create(log, processPath, processID, garden.ProcessIO{}); err != nil {
		return &Process{}, err
	}

	// TODO: Add tests when we come to do Wait for peas
	// This only exists to satisfy integration test (garden server calls wait on a process)
	return &Process{
		log:            log,
		containerID:    sandboxHandle,
		processID:      processID,
		processManager: r.ProcessManager,
	}, nil
}

func (r *RunContainerPea) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	return &Process{}, nil
}

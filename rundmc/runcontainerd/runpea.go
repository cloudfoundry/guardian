package runcontainerd

import (
	"io"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . PeaManager
type PeaManager interface {
	Create(log lager.Logger, bundlePath, id string, io garden.ProcessIO) error
}

type RunContainerPea struct {
	PeaManager     PeaManager
	ProcessManager ProcessManager
}

func (r *RunContainerPea) Run(
	log lager.Logger, processID, processBundlePath, sandboxHandle, sandboxBundlePath string,
	pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
) (garden.Process, error) {

	if err := r.PeaManager.Create(log, processBundlePath, processID, pio); err != nil {
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

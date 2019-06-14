package runcontainerd

import (
	"io"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . PeaManager
type PeaManager interface {
	Create(log lager.Logger, id string, bundle goci.Bndl, io garden.ProcessIO) error
	Delete(log lager.Logger, containerID string) error
	RemoveBundle(log lager.Logger, containerID string) error
}

type RunContainerPea struct {
	PeaManager     PeaManager
	ProcessManager ProcessManager
	BundleSaver    depot.BundleSaver
	ProcessDepot   execrunner.ProcessDepot
}

func (r *RunContainerPea) RunPea(
	log lager.Logger, processID string, processBundle goci.Bndl, sandboxHandle string,
	pio garden.ProcessIO, tty bool, procJSON io.Reader, extraCleanup func() error,
) (garden.Process, error) {

	processBundlePath, err := r.ProcessDepot.CreateProcessDir(log, sandboxHandle, processID)
	if err != nil {
		return nil, err
	}

	err = r.BundleSaver.Save(processBundle, processBundlePath)
	if err != nil {
		return nil, err
	}

	if err := r.PeaManager.Create(log, processID, processBundle, pio); err != nil {
		return &Process{}, err
	}

	return NewPeaProcess(log, processID, r.ProcessManager, r.PeaManager), nil
}

func (r *RunContainerPea) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	return &Process{}, nil
}

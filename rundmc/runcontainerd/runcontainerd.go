package runcontainerd

import (
	"fmt"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
)

type RunContainerd struct{}

func New() *RunContainerd {
	return &RunContainerd{}
}

func (r *RunContainerd) Create(log lager.Logger, bundlePath, id string, io garden.ProcessIO) error {
	return fmt.Errorf("Create not implemented")
}

func (r *RunContainerd) Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	return nil, fmt.Errorf("Exec not implemented")
}

func (r *RunContainerd) Attach(log lager.Logger, bundlePath, id, processId string, io garden.ProcessIO) (garden.Process, error) {
	return nil, fmt.Errorf("Attach not implemented")
}

func (r *RunContainerd) Kill(log lager.Logger, bundlePath string) error {
	return fmt.Errorf("Kill not implemented")
}

func (r *RunContainerd) Delete(log lager.Logger, force bool, bundlePath string) error {
	return fmt.Errorf("Delete not implemented")
}

func (r *RunContainerd) State(log lager.Logger, id string) (runrunc.State, error) {
	return runrunc.State{}, fmt.Errorf("State not implemented")
}

func (r *RunContainerd) Stats(log lager.Logger, id string) (gardener.ActualContainerMetrics, error) {
	return gardener.ActualContainerMetrics{}, fmt.Errorf("Stats not implemented")
}

func (r *RunContainerd) WatchEvents(log lager.Logger, id string, eventsNotifier runrunc.EventsNotifier) error {
	return fmt.Errorf("WatchEvents not implemented")
}

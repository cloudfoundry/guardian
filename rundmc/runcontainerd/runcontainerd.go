package runcontainerd

import (
	"fmt"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . Container
type Container interface {
	containerd.Container
}

//go:generate counterfeiter . Task
type Task interface {
	containerd.Task
}

//go:generate counterfeiter . Nerdulator
type Nerdulator interface {
	CreateContainer(id string, spec specs.Spec) (containerd.Container, error)
	CreateTask(io cio.Creator, container containerd.Container) (containerd.Task, error)
	StartTask(task containerd.Task) error

	State(log lager.Logger, containerID string) (int, containerd.ProcessStatus, error)
	Delete(log lager.Logger, containerID string) error
}

//go:generate counterfeiter . BundleLoader
type BundleLoader interface {
	Load(string) (goci.Bndl, error)
}

type RunContainerd struct {
	nerd         Nerdulator
	bundleLoader BundleLoader
}

func New(nerdulator Nerdulator, bundleLoader BundleLoader) *RunContainerd {
	return &RunContainerd{
		nerd:         nerdulator,
		bundleLoader: bundleLoader,
	}
}

func (r *RunContainerd) Create(log lager.Logger, bundlePath, id string, io garden.ProcessIO) error {
	bundle, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		return err
	}

	container, err := r.nerd.CreateContainer(id, bundle.Spec)
	if err != nil {
		return err
	}

	task, err := r.nerd.CreateTask(cio.NullIO, container)
	if err != nil {
		return err
	}

	return r.nerd.StartTask(task)
}

func (r *RunContainerd) Exec(log lager.Logger, bundlePath, id string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	return nil, fmt.Errorf("Exec is not implemented yet")
}

func (r *RunContainerd) Attach(log lager.Logger, bundlePath, id, processId string, io garden.ProcessIO) (garden.Process, error) {
	return nil, fmt.Errorf("Attach is not implemented yet")
}

func (r *RunContainerd) Kill(log lager.Logger, bundlePath string) error {
	return fmt.Errorf("Kill is not implemented yet")
}

func (r *RunContainerd) Delete(log lager.Logger, force bool, id string) error {
	return r.nerd.Delete(nil, id)
}

func (r *RunContainerd) State(log lager.Logger, id string) (runrunc.State, error) {
	pid, status, err := r.nerd.State(log, id)
	if err != nil {
		return runrunc.State{}, err
	}

	return runrunc.State{Pid: pid, Status: runrunc.Status(status)}, nil
}

func (r *RunContainerd) Stats(log lager.Logger, id string) (gardener.ActualContainerMetrics, error) {
	return gardener.ActualContainerMetrics{}, fmt.Errorf("Stats is not implemented yet")
}

func (r *RunContainerd) WatchEvents(log lager.Logger, id string, eventsNotifier runrunc.EventsNotifier) error {
	return fmt.Errorf("WatchEvents is not implemented yet")
}

package runcontainerd

import (
	"bytes"
	"context"
	"fmt"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
)

type RunContainerd struct {
	client       *containerd.Client
	bundleLoader *goci.BndlLoader
	context      context.Context
}

func New(client *containerd.Client, bundleLoader *goci.BndlLoader, context context.Context) *RunContainerd {
	return &RunContainerd{
		client:       client,
		bundleLoader: bundleLoader,
		context:      context,
	}
}

func (r *RunContainerd) Create(log lager.Logger, bundlePath, id string, io garden.ProcessIO) error {
	bundle, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		return err
	}

	// a "container" in containerd terms is just a bunch of metadata, it does not actually create
	// any running processes at all
	container, err := r.client.NewContainer(r.context, id, containerd.WithSpec(&bundle.Spec))
	if err != nil {
		return err
	}

	// containerd panics if you provide container.NewTask with nil IOs
	// this is a hacky spiketastic workaround
	if io.Stdin == nil || io.Stdout == nil || io.Stderr == nil {
		io.Stdin = bytes.NewBuffer(nil)
		io.Stdout = bytes.NewBuffer(nil)
		io.Stderr = bytes.NewBuffer(nil)
	}

	// container.NewTask essentially does a `runc create`
	_, err = container.NewTask(r.context, cio.NewIO(io.Stdin, io.Stdout, io.Stderr))

	return err
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

func (r *RunContainerd) Delete(log lager.Logger, force bool, id string) error {
	container, err := r.client.LoadContainer(r.context, id)
	if err != nil {
		return err
	}

	task, err := container.Task(r.context, nil)
	if err != nil {
		return err
	}

	if err = task.Kill(r.context, syscall.SIGTERM); err != nil {
		return err
	}

	exitCode, err := task.Wait(r.context)
	if err != nil {
		return err
	}
	<-exitCode

	_, err = task.Delete(r.context)
	if err != nil {
		return err
	}
	return container.Delete(r.context)
}

func (r *RunContainerd) State(log lager.Logger, id string) (runrunc.State, error) {
	container, err := r.client.LoadContainer(r.context, id)
	if err != nil {
		return runrunc.State{}, err
	}

	task, err := container.Task(r.context, nil)
	if err != nil {
		return runrunc.State{}, err
	}

	status, err := task.Status(r.context)
	if err != nil {
		return runrunc.State{}, err
	}

	state := runrunc.State{
		Pid:    int(task.Pid()),
		Status: runrunc.Status(status.Status),
	}

	return state, nil
}

func (r *RunContainerd) Stats(log lager.Logger, id string) (gardener.ActualContainerMetrics, error) {
	return gardener.ActualContainerMetrics{}, fmt.Errorf("Stats not implemented")
}

func (r *RunContainerd) WatchEvents(log lager.Logger, id string, eventsNotifier runrunc.EventsNotifier) error {
	return fmt.Errorf("WatchEvents not implemented")
}

package runcontainerd

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
)

type RunContainerd struct {
	client         *containerd.Client
	bundleLoader   *goci.BndlLoader
	context        context.Context
	processBuilder runrunc.ProcessBuilder
}

func New(client *containerd.Client, bundleLoader *goci.BndlLoader, context context.Context, processBuilder runrunc.ProcessBuilder) *RunContainerd {
	return &RunContainerd{
		client:         client,
		bundleLoader:   bundleLoader,
		context:        context,
		processBuilder: processBuilder,
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
	_, task, err := r.getContainerTask(id)
	if err != nil {
		return nil, err
	}

	bundle, err := r.bundleLoader.Load(bundlePath)
	if err != nil {
		return nil, err
	}

	if spec.Dir == "" {
		// We hardcode the CWD to /root jsut for the POC. In reality this should resolve to user home
		spec.Dir = "/root"
	}

	preparedSpec := r.processBuilder.BuildProcess(bundle, runrunc.ProcessSpec{
		ProcessSpec: spec,
	})

	// We hardcode the task ID to potato, in reality process ID generator should be used (see runrunc)
	process, err := task.Exec(r.context, "potato", &preparedSpec.Process, cio.NewIO(io.Stdin, io.Stdout, io.Stderr))
	if err != nil {
		return nil, err
	}

	if err := process.Start(r.context); err != nil {
		return nil, fmt.Errorf("hey %s", err)
	}

	return newGardenProcess(r.context, process), nil
}

func (r *RunContainerd) Attach(log lager.Logger, bundlePath, id, processId string, io garden.ProcessIO) (garden.Process, error) {
	return nil, fmt.Errorf("Attach not implemented")
}

func (r *RunContainerd) Kill(log lager.Logger, bundlePath string) error {
	return fmt.Errorf("Kill not implemented")
}

func (r *RunContainerd) Delete(log lager.Logger, force bool, id string) error {
	container, task, err := r.getContainerTask(id)
	if err != nil {
		return err
	}

	_, err = task.Delete(r.context, containerd.WithProcessKill)
	if err != nil {
		return err
	}
	return container.Delete(r.context)
}

func (r *RunContainerd) State(log lager.Logger, id string) (runrunc.State, error) {

	_, task, err := r.getContainerTask(id)
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

func (r *RunContainerd) getContainerTask(containerID string) (containerd.Container, containerd.Task, error) {
	container, err := r.client.LoadContainer(r.context, containerID)
	if err != nil {
		return nil, nil, err
	}

	task, err := container.Task(r.context, nil)
	if err != nil {
		return container, nil, err
	}
	return container, task, nil
}

type ContainerdToGardenProcessAdapter struct {
	containerdProcess containerd.Process
	context           context.Context
}

func newGardenProcess(context context.Context, process containerd.Process) *ContainerdToGardenProcessAdapter {
	return &ContainerdToGardenProcessAdapter{containerdProcess: process, context: context}
}

func (w *ContainerdToGardenProcessAdapter) ID() string {
	return fmt.Sprint(w.containerdProcess.Pid())
}

func (w *ContainerdToGardenProcessAdapter) Wait() (int, error) {
	exitStatusChan, err := w.containerdProcess.Wait(w.context)
	exitStatus := <-exitStatusChan
	return int(exitStatus.ExitCode()), err
}
func (w *ContainerdToGardenProcessAdapter) SetTTY(garden.TTYSpec) error {
	return errors.New("SetTTY is not implemented")
}

func (w *ContainerdToGardenProcessAdapter) Signal(garden.Signal) error {
	return errors.New("Signal is not implemented")
}

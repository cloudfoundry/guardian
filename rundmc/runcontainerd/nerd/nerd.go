package nerd

import (
	"context"

	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// TODO: I don't think we should use the same context repeatedly?
type Nerd struct {
	client  *containerd.Client
	context context.Context
}

func New(client *containerd.Client, context context.Context) *Nerd {
	return &Nerd{
		client:  client,
		context: context,
	}
}

func (n *Nerd) Create(log lager.Logger, containerID string, spec *specs.Spec) error {
	container, err := n.client.NewContainer(n.context, containerID, containerd.WithSpec(spec))
	if err != nil {
		return err
	}

	task, err := container.NewTask(n.context, cio.NullIO)
	if err != nil {
		return err
	}

	return task.Start(n.context)
}

func (n *Nerd) Delete(log lager.Logger, containerID string) error {
	container, err := n.client.LoadContainer(n.context, containerID)
	if err != nil {
		return err
	}

	task, err := container.Task(n.context, nil)
	if err != nil {
		return err
	}

	_, err = task.Delete(n.context, containerd.WithProcessKill)
	if err != nil {
		return err
	}

	return container.Delete(n.context)
}

func (n *Nerd) State(log lager.Logger, containerID string) (int, containerd.ProcessStatus, error) {
	container, err := n.client.LoadContainer(n.context, containerID)
	if err != nil {
		return 0, "", err
	}

	task, err := container.Task(n.context, nil)
	if err != nil {
		return 0, "", err
	}

	status, err := task.Status(n.context)
	if err != nil {
		return 0, "", err
	}

	return int(task.Pid()), status.Status, nil
}

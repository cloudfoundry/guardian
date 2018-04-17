package nerd

import (
	"context"

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

func (n *Nerd) CreateContainer(id string, spec specs.Spec) (containerd.Container, error) {
	return n.client.NewContainer(n.context, id, containerd.WithSpec(&spec))
}

func (n *Nerd) CreateTask(io cio.Creator, container containerd.Container) (containerd.Task, error) {
	return container.NewTask(n.context, io)
}

func (n *Nerd) StartTask(task containerd.Task) error {
	return task.Start(n.context)
}

func (n *Nerd) LoadContainer(id string) (containerd.Container, error) {
	return n.client.LoadContainer(n.context, id)
}

func (n *Nerd) GetTask(container containerd.Container) (containerd.Task, error) {
	return container.Task(n.context, nil)
}

func (n *Nerd) GetTaskPid(task containerd.Task) int {
	return int(task.Pid())
}

package nerd

import (
	"context"
	"strconv"

	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/linux/runctypes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

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

func WithNoNewKeyring(ctx context.Context, c *containerd.Client, ti *containerd.TaskInfo) error {
	ti.Options = &runctypes.CreateOptions{NoNewKeyring: true}
	return nil
}

func (n *Nerd) Create(log lager.Logger, containerID string, spec *specs.Spec) error {
	log.Debug("creating-container", lager.Data{"id": containerID})
	container, err := n.client.NewContainer(n.context, containerID, containerd.WithSpec(spec))
	if err != nil {
		return err
	}

	log.Debug("creating-task", lager.Data{"id": containerID})
	task, err := container.NewTask(n.context, cio.NullIO, WithNoNewKeyring)
	if err != nil {
		return err
	}

	log.Debug("starting-task", lager.Data{"id": containerID})
	return task.Start(n.context)
}

func (n *Nerd) Delete(log lager.Logger, containerID string) error {
	log.Debug("loading-container", lager.Data{"id": containerID})
	container, err := n.client.LoadContainer(n.context, containerID)
	if err != nil {
		return err
	}

	log.Debug("loading-task", lager.Data{"id": containerID})
	task, err := container.Task(n.context, nil)
	if err != nil {
		return err
	}

	log.Debug("deleting-task", lager.Data{"id": containerID})
	_, err = task.Delete(n.context, containerd.WithProcessKill)
	if err != nil {
		return err
	}

	log.Debug("deleting-container", lager.Data{"id": containerID})
	return container.Delete(n.context)
}

func (n *Nerd) State(log lager.Logger, containerID string) (int, containerd.ProcessStatus, error) {
	log.Debug("loading-container", lager.Data{"id": containerID})
	container, err := n.client.LoadContainer(n.context, containerID)
	if err != nil {
		return 0, "", err
	}

	log.Debug("loading-task", lager.Data{"id": containerID})
	task, err := container.Task(n.context, nil)
	if err != nil {
		return 0, "", err
	}

	log.Debug("getting-task-status", lager.Data{"id": containerID})
	status, err := task.Status(n.context)
	if err != nil {
		return 0, "", err
	}

	log.Debug("task-result", lager.Data{"id": containerID, "pid": strconv.Itoa(int(task.Pid())), "status": string(status.Status)})
	return int(task.Pid()), status.Status, nil
}

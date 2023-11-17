//go:build !windows

package nerd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/lager/v3"
	"github.com/containerd/containerd"
	apievents "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	ctrdevents "github.com/containerd/containerd/events"
	v2types "github.com/containerd/containerd/runtime/v2/runc/options"
	"github.com/containerd/typeurl/v2"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Nerd struct {
	client    *containerd.Client
	context   context.Context
	ioFifoDir string
}

func New(client *containerd.Client, context context.Context, ioFifoDir string) *Nerd {
	return &Nerd{
		client:    client,
		context:   context,
		ioFifoDir: ioFifoDir,
	}
}

func (n *Nerd) Create(log lager.Logger, containerID string, spec *specs.Spec, hostUID, hostGID uint32, pio func() (io.Reader, io.Writer, io.Writer)) error {
	log.Debug("creating-container", lager.Data{"containerID": containerID})
	container, err := n.client.NewContainer(n.context, containerID, containerd.WithSpec(spec))
	if err != nil {
		return err
	}

	for key, value := range spec.Annotations {
		log.Debug("setting-label", lager.Data{"containerID": containerID, "label-key": key, "label-value": value})
		if _, err := container.SetLabels(n.context, map[string]string{key: value}); err != nil {
			return err
		}
	}

	log.Debug("creating-task", lager.Data{"containerID": containerID})
	task, err := container.NewTask(n.context, cio.NewCreator(withProcessIO(noTTYProcessIO(pio), n.ioFifoDir)), containerd.WithNoNewKeyring, WithUIDAndGID(hostUID, hostGID))
	if err != nil {
		return err
	}

	log.Debug("starting-task", lager.Data{"containerID": containerID})
	err = task.Start(n.context)
	if err != nil {
		return err
	}

	return task.CloseIO(n.context, containerd.WithStdinCloser)
}

func withProcessKillLogging(log lager.Logger) func(context.Context, containerd.Process) error {
	return func(ctx context.Context, p containerd.Process) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		// ignore errors to wait and kill as we are forcefully killing
		// the process and don't care about the exit status
		log.Debug("with-process-kill.wait", lager.Data{"containerdProcess": p.ID(), "processID": p.Pid()})
		s, err := p.Wait(ctx)
		if err != nil {
			return err
		}
		log.Debug("with-process-kill.kill", lager.Data{"containerdProcess": p.ID(), "processID": p.Pid()})
		if err := p.Kill(ctx, syscall.SIGKILL); err != nil {
			if errdefs.IsFailedPrecondition(err) || errdefs.IsNotFound(err) {
				return nil
			}
			return err
		}
		log.Debug("with-process-kill.kill-complete", lager.Data{"containerdProcess": p.ID(), "processID": p.Pid()})
		// wait for the process to fully stop before letting the rest of the deletion complete
		select {
		case <-s:
			break
		case <-time.After(time.Minute * 2):
			return fmt.Errorf("timed out waiting for container kill: containerdProcess=%s, processID=%d", p.ID(), p.Pid())
		}

		log.Debug("with-process-kill.wait-complete", lager.Data{"containerdProcess": p.ID(), "processID": p.Pid()})
		return nil
	}
}

func (n *Nerd) Delete(log lager.Logger, containerID string) error {
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		switch err.(type) {
		case runcontainerd.ContainerNotFoundError:
			log.Debug("container-already-deleted", lager.Data{"containerID": containerID})
			return nil
		case runcontainerd.TaskNotFoundError:
			log.Debug("task-already-deleted", lager.Data{"containerID": containerID})
			return nil
		}
		return err
	}

	log.Debug("deleting-task", lager.Data{"containerID": containerID})
	_, err = task.Delete(n.context, withProcessKillLogging(log))
	return err
}

func (n *Nerd) State(log lager.Logger, containerID string) (int, string, error) {
	log = log.Session("state", lager.Data{"containerID": containerID})

	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return 0, "", err
	}

	log.Debug("getting-task-status")
	status, err := n.tryToGetStatus(log, task)
	if err != nil {
		return 0, "", err
	}

	log.Debug("task-result", lager.Data{"pid": strconv.Itoa(int(task.Pid())), "status": string(status.Status)})
	return int(task.Pid()), string(status.Status), nil
}

func (n *Nerd) tryToGetTask(log lager.Logger, container containerd.Container) (containerd.Task, error) {
	log = log.Session("try-to-get-task")

	const retries = 5
	for i := 0; i < retries; i++ {
		task, err := container.Task(n.context, cio.Load)
		if err != nil {
			if errdefs.IsNotFound(err) {
				return nil, runcontainerd.TaskNotFoundError{Handle: container.ID()}
			}
			return nil, err
		}

		if task.Pid() != 0 {
			return task, nil
		}

		log.Info("retrying-after-pid-0-response", lager.Data{"retry-number": i + 1})
		time.Sleep(500 * time.Millisecond)
	}

	return nil, errors.New("failed getting task")
}

func (n *Nerd) tryToGetStatus(log lager.Logger, task containerd.Task) (containerd.Status, error) {
	log = log.Session("try-to-get-status", lager.Data{"taskID": task.ID()})

	const retries = 5
	for i := 0; i < retries; i++ {
		status, err := task.Status(n.context)
		if err != nil {
			return containerd.Status{}, err
		}

		if status.Status != containerd.Unknown {
			return status, nil
		}

		log.Info("retrying-after-status-unknown-response", lager.Data{"retry-number": i + 1})
		time.Sleep(500 * time.Millisecond)
	}

	return containerd.Status{}, errors.New("failed getting task status")
}

func (n *Nerd) Exec(log lager.Logger, containerID, processID string, spec *specs.Process, processIO func() (io.Reader, io.Writer, io.Writer, bool)) (runcontainerd.BackingProcess, error) {
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return BackingProcess{}, err
	}

	log.Debug("execing-task", lager.Data{"containerID": containerID, "processID": processID})
	process, err := task.Exec(n.context, processID, spec, cio.NewCreator(withProcessIO(processIO, n.ioFifoDir)))
	if err != nil {
		return BackingProcess{}, err
	}

	log.Debug("starting-task", lager.Data{"containerID": containerID, "processID": processID})
	if err := process.Start(n.context); err != nil {
		return BackingProcess{}, err
	}

	log.Debug("closing-stdin", lager.Data{"containerID": containerID, "processID": processID})
	go exponentialBackoffCloseIO(process, n.context, log, containerID)

	return NewBackingProcess(log, process, n.context), nil
}

func (n *Nerd) GetProcess(log lager.Logger, containerID, processID string) (runcontainerd.BackingProcess, error) {
	log.Debug("get-process", lager.Data{"containerID": containerID, "processID": processID})
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return BackingProcess{}, err
	}

	process, err := task.LoadProcess(n.context, processID, cio.Load)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return BackingProcess{}, runcontainerd.ProcessNotFoundError{Handle: containerID, ID: processID}
		}
		return BackingProcess{}, err
	}

	return NewBackingProcess(log, process, n.context), nil
}

func (n *Nerd) GetTask(log lager.Logger, id string) (runcontainerd.BackingProcess, error) {
	log.Debug("get-task", lager.Data{"id": id})
	_, task, err := n.loadContainerAndTask(log, id)
	if err != nil {
		return BackingProcess{}, err
	}

	return NewBackingProcess(log, task, n.context), nil
}

func exponentialBackoffCloseIO(process containerd.Process, ctx context.Context, log lager.Logger, containerID string) {
	duration := 3 * time.Second
	retries := 10
	for i := 0; i < retries; i++ {
		if err := process.CloseIO(ctx, containerd.WithStdinCloser); err != nil {
			log.Error("failed-closing-stdin", err, lager.Data{"containerID": containerID, "processID": process.ID()})
			time.Sleep(duration)
			duration *= 2
			continue
		}
		break
	}
}

func noTTYProcessIO(initProcessIO func() (io.Reader, io.Writer, io.Writer)) func() (io.Reader, io.Writer, io.Writer, bool) {
	return func() (io.Reader, io.Writer, io.Writer, bool) {
		stdin, stdout, stderr := initProcessIO()
		return stdin, stdout, stderr, false
	}
}

func withProcessIO(processIO func() (io.Reader, io.Writer, io.Writer, bool), ioFifoDir string) cio.Opt {
	return func(opt *cio.Streams) {
		stdin, stdout, stderr, hasTTY := processIO()

		cio.WithFIFODir(ioFifoDir)(opt)
		if hasTTY {
			cio.WithTerminal(opt)
			cio.WithStreams(orEmpty(stdin), orDiscard(stdout), nil)(opt)
		} else {
			cio.WithStreams(orEmpty(stdin), orDiscard(stdout), orDiscard(stderr))(opt)
		}
	}
}

func orEmpty(r io.Reader) io.Reader {
	if r != nil {
		return r
	}
	return bytes.NewBuffer(nil)
}

func orDiscard(w io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return io.Discard
}

func (n *Nerd) GetContainerPID(log lager.Logger, containerID string) (uint32, error) {
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return 0, err
	}

	return task.Pid(), nil
}

func (n *Nerd) loadContainer(log lager.Logger, containerID string) (containerd.Container, error) {
	log.Debug("loading-container", lager.Data{"containerID": containerID})
	container, err := n.client.LoadContainer(n.context, containerID)
	if err != nil {
		if errdefs.IsNotFound(err) {
			log.Debug("container-not-found", lager.Data{"containerID": containerID})
			return nil, runcontainerd.ContainerNotFoundError{Handle: containerID}
		}
		log.Debug("loading-container-failed", lager.Data{"containerID": containerID})
		return nil, err
	}
	return container, nil
}

func (n *Nerd) loadContainers(labels ...runcontainerd.ContainerFilter) ([]containerd.Container, error) {
	var flattenedLabels []string
	for _, label := range labels {
		flattenedLabels = append(flattenedLabels, fmt.Sprintf("labels.\"%s\"%s%s", label.Label, label.ComparisonOp, label.Value))
	}

	return n.client.Containers(n.context, strings.Join(flattenedLabels, ","))
}

func (n *Nerd) loadContainerAndTask(log lager.Logger, containerID string) (containerd.Container, containerd.Task, error) {
	container, err := n.loadContainer(log, containerID)
	if err != nil {
		return nil, nil, err
	}

	log.Debug("loading-task", lager.Data{"containerID": containerID})
	task, err := n.tryToGetTask(log, container)
	if err != nil {
		log.Debug("loading-task-failed", lager.Data{"containerID": containerID})
		return nil, nil, err
	}

	return container, task, nil
}

func (n *Nerd) OOMEvents(log lager.Logger) <-chan *apievents.TaskOOM {
	events, errs := n.client.Subscribe(n.context, `topic=="/tasks/oom"`)
	oomEvents := make(chan *apievents.TaskOOM)

	go func() {
		for {
			select {
			case err, ok := <-errs:
				if !ok {
					log.Info("event service has been closed")
				}

				if err != nil {
					log.Error("event service received an error", err)
				}

				close(oomEvents)
				return

			case e := <-events:
				event, err := coerceEvent(e)
				if err != nil {
					log.Error("failed to coerce containerd event", err, lager.Data{"event": e})
					continue
				}

				log.Debug("received an OOM event", lager.Data{"containerID": event.ContainerID})
				oomEvents <- event
			}
		}
	}()

	return oomEvents
}

func (n *Nerd) Spec(log lager.Logger, containerID string) (*specs.Spec, error) {
	container, _, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return nil, err
	}

	log.Debug("getting-container-spec", lager.Data{"containerID": containerID})
	return container.Spec(n.context)
}

func coerceEvent(event *ctrdevents.Envelope) (*apievents.TaskOOM, error) {
	if event.Event == nil {
		return nil, errors.New("empty event")
	}

	unmarshalledEvent, err := typeurl.UnmarshalAny(event.Event)
	if err != nil {
		return nil, err
	}

	oom, ok := unmarshalledEvent.(*apievents.TaskOOM)
	if !ok {
		return nil, errors.New("unexpected event")
	}

	return oom, nil
}

func (n *Nerd) BundleIDs(labels ...runcontainerd.ContainerFilter) ([]string, error) {
	containers, err := n.loadContainers(labels...)
	if err != nil {
		return nil, err
	}

	handles := []string{}
	for _, container := range containers {
		handles = append(handles, container.ID())
	}

	return handles, nil
}

func (n *Nerd) RemoveBundle(log lager.Logger, handle string) error {
	container, err := n.loadContainer(log, handle)
	if err == nil {
		log.Debug("deleting-container", lager.Data{"containerID": handle})
		return container.Delete(n.context)
	}

	if _, isNotFound := err.(runcontainerd.ContainerNotFoundError); isNotFound {
		log.Debug("container-already-deleted", lager.Data{"containerID": handle})
		return nil
	}

	log.Debug("loading-container-failed", lager.Data{"handle": handle})
	return err
}

func WithUIDAndGID(uid, gid uint32) containerd.NewTaskOpts {
	return func(ctx context.Context, c *containerd.Client, ti *containerd.TaskInfo) error {
		return updateTaskInfoCreateOptions(ti, func(opts *v2types.Options) error {
			opts.IoUid = uid
			opts.IoGid = gid
			return nil
		})
	}
}

func updateTaskInfoCreateOptions(taskInfo *containerd.TaskInfo, updateCreateOptions func(createOptions *v2types.Options) error) error {
	if taskInfo.Options == nil {
		taskInfo.Options = &v2types.Options{}
	}
	opts, ok := taskInfo.Options.(*v2types.Options)

	if !ok {
		return errors.New("could not cast TaskInfo Options to CreateOptions")
	}

	return updateCreateOptions(opts)
}

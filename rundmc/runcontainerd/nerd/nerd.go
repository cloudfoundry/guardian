package nerd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/runtime/linux/runctypes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Nerd struct {
	client  *containerd.Client
	context context.Context
	fifoDir string
}

func New(client *containerd.Client, context context.Context) *Nerd {
	// We need to override this value. If we don't, containerd will try to mkdir /run/containerd and fail with Persmission denided.
	return &Nerd{
		client:  client,
		context: context,
		fifoDir: "/var/vcap/data/containerd/runc/user/4294967294/fifos",
	}
}

func (n *Nerd) Create(log lager.Logger, containerID string, spec *specs.Spec, pio func() (io.Reader, io.Writer, io.Writer)) error {
	log.Debug("creating-container", lager.Data{"containerID": containerID})
	container, err := n.client.NewContainer(n.context, containerID, containerd.WithSpec(spec))
	if err != nil {
		return err
	}

	log.Debug("creating-task", lager.Data{"containerID": containerID})
	task, err := container.NewTask(n.context, cio.NewCreator(withProcessIO(pio), cio.WithFIFODir(n.fifoDir)), containerd.WithNoNewKeyring, WithCurrentUIDAndGID)
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

func WithCurrentUIDAndGID(ctx context.Context, client *containerd.Client, taskInfo *containerd.TaskInfo) error {
	if taskInfo.Options == nil {
		taskInfo.Options = &runctypes.CreateOptions{}
	}

	opts, ok := taskInfo.Options.(*runctypes.CreateOptions)
	if !ok {
		return errors.New("could not cast TaskInfo Options to CreateOptions")
	}

	opts.IoUid = uint32(syscall.Geteuid())
	opts.IoGid = uint32(syscall.Getegid())

	return nil
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
		if err := p.Kill(ctx, syscall.SIGKILL, containerd.WithKillAll); err != nil {
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
	container, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		switch err.(type) {
		case *ContainerNotFoundError:
			return nil
		case *TaskNotFoundError:
			log.Debug("deleting-container", lager.Data{"containerID": containerID})
			return container.Delete(n.context)
		}
		return err
	}

	log.Debug("deleting-task", lager.Data{"containerID": containerID})
	_, err = task.Delete(n.context, withProcessKillLogging(log))
	if err != nil {
		return err
	}

	log.Debug("deleting-container", lager.Data{"containerID": containerID})
	return container.Delete(n.context)
}

func (n *Nerd) State(log lager.Logger, containerID string) (int, string, error) {
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return 0, "", err
	}

	log.Debug("getting-task-status", lager.Data{"containerID": containerID})
	status, err := task.Status(n.context)
	if err != nil {
		return 0, "", err
	}

	log.Debug("task-result", lager.Data{"containerID": containerID, "pid": strconv.Itoa(int(task.Pid())), "status": string(status.Status)})
	return int(task.Pid()), string(status.Status), nil
}

func (n *Nerd) Exec(log lager.Logger, containerID, processID string, spec *specs.Process, processIO func() (io.Reader, io.Writer, io.Writer)) error {
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return err
	}

	log.Debug("execing-task", lager.Data{"containerID": containerID, "processID": processID})
	process, err := task.Exec(n.context, processID, spec, cio.NewCreator(withProcessIO(processIO), cio.WithFIFODir(n.fifoDir)))
	if err != nil {
		return err
	}

	log.Debug("starting-task", lager.Data{"containerID": containerID, "processID": processID})
	if err := process.Start(n.context); err != nil {
		return err
	}

	log.Debug("closing-stdin", lager.Data{"containerID": containerID, "processID": processID})
	return process.CloseIO(n.context, containerd.WithStdinCloser)
}

func (n *Nerd) DeleteProcess(log lager.Logger, containerID, processID string) error {
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return err
	}

	process, err := task.LoadProcess(n.context, processID, nil)
	if err != nil {
		return err
	}

	_, err = process.Delete(n.context)
	if err != nil {
		return err
	}

	return nil
}

func withProcessIO(processIO func() (io.Reader, io.Writer, io.Writer)) cio.Opt {
	return func(opt *cio.Streams) {
		stdin, stdout, stderr := processIO()
		cio.WithStreams(orEmpty(stdin), orDiscard(stdout), orDiscard(stderr))(opt)
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
	return ioutil.Discard
}

func (n *Nerd) GetContainerPID(log lager.Logger, containerID string) (uint32, error) {
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return 0, err
	}

	return task.Pid(), nil
}

func (n *Nerd) loadContainerAndTask(log lager.Logger, containerID string) (containerd.Container, containerd.Task, error) {
	var err error

	log.Debug("loading-container", lager.Data{"containerID": containerID})
	container, err := n.client.LoadContainer(n.context, containerID)
	if errdefs.IsNotFound(err) {
		log.Debug("container-not-found", lager.Data{"containerID": containerID})
		return nil, nil, &ContainerNotFoundError{Handle: containerID}
	}

	log.Debug("loading-task", lager.Data{"containerID": containerID})
	task, err := container.Task(n.context, nil)
	if errdefs.IsNotFound(err) {
		log.Debug("task-not-found", lager.Data{"containerID": containerID})
		return container, nil, &TaskNotFoundError{Handle: containerID}
	}

	return container, task, err
}

func (n *Nerd) Wait(log lager.Logger, containerID, processID string) (int, error) {
	log.Debug("waiting-on-process", lager.Data{"containerID": containerID, "processID": processID})
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return 0, err
	}

	process, err := task.LoadProcess(n.context, processID, nil)
	if err != nil {
		return 0, err
	}

	exitCh, err := process.Wait(n.context)
	if err != nil {
		return 0, err
	}

	// Containerd might fail to retrieve the ExitCode for non-process related reasons
	exitStatus := <-exitCh
	if exitStatus.Error() != nil {
		return 0, exitStatus.Error()
	}

	return int(exitStatus.ExitCode()), nil
}

func (n *Nerd) Signal(log lager.Logger, containerID, processID string, signal syscall.Signal) error {
	log.Debug("signalling-process", lager.Data{"containerID": containerID, "processID": processID, "signal": signal})
	_, task, err := n.loadContainerAndTask(log, containerID)
	if err != nil {
		return err
	}

	process, err := task.LoadProcess(n.context, processID, nil)
	if err != nil {
		return err
	}

	return process.Kill(n.context, signal)
}

package nerd

import (
	"context"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
)

//go:generate counterfeiter github.com/containerd/containerd.Process
//go:generate counterfeiter github.com/containerd/containerd/cio.IO

type BackingProcess struct {
	log               lager.Logger
	context           context.Context
	containerdProcess containerd.Process
}

func NewBackingProcess(log lager.Logger, p containerd.Process, ctx context.Context) BackingProcess {
	return BackingProcess{
		log:               log,
		context:           ctx,
		containerdProcess: p,
	}
}

func (p BackingProcess) ID() string {
	return p.containerdProcess.ID()
}

func (p BackingProcess) Wait() (int, error) {
	exitCh, err := p.containerdProcess.Wait(p.context)
	if err != nil {
		return 0, err
	}

	exitStatus := <-exitCh
	if exitStatus.Error() != nil {
		return 0, exitStatus.Error()
	}

	p.containerdProcess.IO().Wait()
	return int(exitStatus.ExitCode()), nil
}

func (p BackingProcess) Signal(signal syscall.Signal) error {
	return p.containerdProcess.Kill(p.context, signal)
}

func (p BackingProcess) Delete() error {
	_, err := p.containerdProcess.Delete(p.context)
	return err
}

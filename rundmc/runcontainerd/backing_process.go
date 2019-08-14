package runcontainerd

import (
	"context"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
)

type backingProcess struct {
	log               lager.Logger
	context           context.Context
	containerdProcess containerd.Process
}

func NewBackingProcess(log lager.Logger, p containerd.Process, ctx context.Context) *backingProcess {
	return &backingProcess{
		log:               log,
		context:           ctx,
		containerdProcess: p,
	}
}

func (p *backingProcess) ID() string {
	return p.containerdProcess.ID()
}

func (p *backingProcess) Wait() (int, error) {
	exitCh, err := p.containerdProcess.Wait(p.context)
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

func (p *backingProcess) Signal(signal syscall.Signal) error {
	return p.containerdProcess.Kill(p.context, signal)
}

func (p *backingProcess) Delete() error {
	_, err := p.containerdProcess.Delete(p.context)
	return err
}

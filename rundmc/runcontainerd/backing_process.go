package runcontainerd

import (
	"context"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
)

type backingProcess struct {
	log                      lager.Logger
	context                  context.Context
	containerdProcess        containerd.Process
	cleanupProcessDirsOnWait bool
}

func NewBackingProcess(log lager.Logger, p containerd.Process, ctx context.Context, cleanupProcessDirsOnWait bool) *backingProcess {
	return &backingProcess{
		log:                      log,
		context:                  ctx,
		containerdProcess:        p,
		cleanupProcessDirsOnWait: cleanupProcessDirsOnWait,
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

	if p.cleanupProcessDirsOnWait {
		p.log.Debug("wait.cleanup-process", lager.Data{"processID": p.containerdProcess.ID()})
		_, err = p.containerdProcess.Delete(p.context)
		if err != nil {
			p.log.Error("cleanup-failed-deleting-process", err)
		}
	}

	return int(exitStatus.ExitCode()), nil
}

func (p *backingProcess) Signal(signal syscall.Signal) error {
	return p.containerdProcess.Kill(p.context, signal)
}

package runcontainerd

import (
	"code.cloudfoundry.org/lager"
)

type PeaProcess struct {
	Process
	peaManager PeaManager
}

func NewPeaProcess(log lager.Logger, processID string, processManager ProcessManager, peaManager PeaManager) *PeaProcess {
	return &PeaProcess{
		Process:    *NewProcess(log, processID, processID, processManager),
		peaManager: peaManager,
	}
}

func (p *PeaProcess) Wait() (int, error) {
	exitCode, err := p.Process.Wait()
	if err != nil {
		return 0, err
	}

	if err := p.peaManager.Delete(p.log, p.containerID); err != nil {
		return 0, err
	}

	if err := p.peaManager.RemoveBundle(p.log, p.containerID); err != nil {
		return 0, err
	}

	return exitCode, nil
}

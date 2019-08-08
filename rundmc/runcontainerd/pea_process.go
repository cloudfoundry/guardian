package runcontainerd

import (
	"code.cloudfoundry.org/lager"
)

type PeaProcess struct {
	Process
	peaManager PeaManager
	volumizer  Volumizer
}

func NewPeaProcess(log lager.Logger, backingProcess BackingProcess, peaManager PeaManager, volumizer Volumizer) *PeaProcess {
	return &PeaProcess{
		Process:    *NewProcess(log, backingProcess),
		peaManager: peaManager,
		volumizer:  volumizer,
	}
}

func (p *PeaProcess) Wait() (int, error) {
	p.log.Debug("pea-wait", lager.Data{"id": p.Process.ID()})
	exitCode, err := p.Process.Wait()
	if err != nil {
		return 0, err
	}

	if err := p.peaManager.Delete(p.log, p.ID()); err != nil {
		return 0, err
	}

	if err := p.peaManager.RemoveBundle(p.log, p.ID()); err != nil {
		return 0, err
	}

	if err := p.volumizer.Destroy(p.log, p.ID()); err != nil {
		return 0, err
	}

	return exitCode, nil
}

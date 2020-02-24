package gardener

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

type peaProcess struct {
	logger             lager.Logger
	gardenProcess      garden.Process
	peaContainerHandle string
	gardener           *Gardener
}

func NewPeaProcess(logger lager.Logger, gardenProcess garden.Process, peaContainerHandle string, gardener *Gardener) *peaProcess {
	return &peaProcess{
		logger:             logger,
		gardenProcess:      gardenProcess,
		peaContainerHandle: peaContainerHandle,
		gardener:           gardener,
	}
}

func (p peaProcess) ID() string {
	return p.peaContainerHandle
}

func (p peaProcess) Wait() (int, error) {
	// log := p.logger.Session("pea-process.wait")

	exitCode, err := p.gardenProcess.Wait()
	if err != nil {
		if err := p.gardener.destroy(p.logger, p.peaContainerHandle); err != nil {
			p.logger.Error("process-cleanup", err)
		}
		return -1, err
	}

	p.logger.Debug("pea-wait", lager.Data{"peaContainerHandle": p.peaContainerHandle})
	if err := p.gardener.destroy(p.logger, p.peaContainerHandle); err != nil {
		p.logger.Error("process-cleanup", err)
	}

	return exitCode, nil
}

func (p peaProcess) SetTTY(ttySpec garden.TTYSpec) error {
	return p.gardenProcess.SetTTY(ttySpec)
}

func (p peaProcess) Signal(signal garden.Signal) error {
	return p.gardenProcess.Signal(signal)
}

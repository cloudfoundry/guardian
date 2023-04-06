package runcontainerd

import (
	"code.cloudfoundry.org/lager/v3"
)

type PidGetter struct {
	Nerd ContainerManager
}

func (p *PidGetter) GetPid(logger lager.Logger, containerID string) (int, error) {
	pid, _, err := p.Nerd.State(logger, containerID)
	return pid, err
}

func (p *PidGetter) GetPeaPid(logger lager.Logger, _, peaID string) (int, error) {
	return p.GetPid(logger, peaID)
}

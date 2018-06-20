package runcontainerd

import (
	"code.cloudfoundry.org/lager"
)

type PidGetter struct {
	Nerd ContainerManager
}

func (p *PidGetter) GetPid(logger lager.Logger, containerHandle string) (int, error) {
	pid, _, err := p.Nerd.State(logger, containerHandle)
	return pid, err
}

package privchecker

import (
	"code.cloudfoundry.org/lager"
	"github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate counterfeiter . ContainerManager
type ContainerManager interface {
	Spec(log lager.Logger, containerID string) (*specs.Spec, error)
}

type PrivilegeChecker struct {
	Log              lager.Logger
	ContainerManager ContainerManager
}

func (p *PrivilegeChecker) Privileged(id string) (bool, error) {
	spec, err := p.ContainerManager.Spec(p.Log, id)
	if err != nil {
		return false, err
	}

	for _, namespace := range spec.Linux.Namespaces {
		if namespace.Type == "user" {
			return false, nil
		}
	}

	return true, nil
}

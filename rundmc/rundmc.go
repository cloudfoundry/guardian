package rundmc

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

//go:generate counterfeiter . Depot
type Depot interface {
	Create(handle string) error
}

type Containerizer struct {
	Depot Depot
}

func (c *Containerizer) Create(spec gardener.DesiredContainerSpec) error {
	c.Depot.Create(spec.Handle)
	return nil
}

func (c *Containerizer) Run(handle string, spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	panic("not implemented")
	return nil, nil
}

package kawasaki

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/pivotal-golang/lager"
)

type CompositeNetworker struct {
	Networker
	ExtraHooks []NetworkHooker
}

func (c *CompositeNetworker) Hooks(log lager.Logger, containerSpec garden.ContainerSpec) ([]gardener.Hooks, error) {
	hooks, err := c.Networker.Hooks(log, containerSpec)
	if err != nil {
		return []gardener.Hooks{}, err
	}

	for _, hooker := range c.ExtraHooks {
		hook, err := hooker.Hooks(log, containerSpec)
		if err != nil {
			return []gardener.Hooks{}, err
		}

		hooks = append(hooks, hook)
	}

	return hooks, nil
}

package guardiancmd

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager"
)

func (cmd *ServerCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	return gardener.NoopVolumeCreator{}
}

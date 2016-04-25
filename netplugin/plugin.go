package netplugin

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/pivotal-golang/lager"
)

type Plugin struct {
	path     string
	extraArg []string
}

func New(path string, extraArg ...string) *Plugin {
	return &Plugin{
		path:     path,
		extraArg: extraArg,
	}
}

func (p Plugin) Hooks(log lager.Logger, containerSpec garden.ContainerSpec) (gardener.Hooks, error) {
	pathAndExtraArgs := append([]string{p.path}, p.extraArg...)

	handle := containerSpec.Handle
	spec := containerSpec.Network
	externalSpec := containerSpec.Properties[gardener.ExternalNetworkSpecKey]

	networkPluginFlags := []string{"--handle", handle, "--network", spec, "--external-network", externalSpec}

	upArgs := append(pathAndExtraArgs, "--action", "up")
	upArgs = append(upArgs, networkPluginFlags...)
	downArgs := append(pathAndExtraArgs, "--action", "down")
	downArgs = append(downArgs, networkPluginFlags...)

	return gardener.Hooks{
		Prestart: gardener.Hook{
			Path: p.path,
			Args: upArgs,
		},
		Poststop: gardener.Hook{
			Path: p.path,
			Args: downArgs,
		},
	}, nil
}

package netplugin

import (
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

func (p Plugin) Hooks(log lager.Logger, handle, spec string) (gardener.Hooks, error) {
	pathAndExtraArgs := append([]string{p.path}, p.extraArg...)
	networkPluginFlags := []string{"--handle", handle, "--network", spec}

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

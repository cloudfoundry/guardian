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

func (p Plugin) Hooks(log lager.Logger, handle, spec string) (gardener.Hooks, error) {
	var args []string
	args = append(args, p.extraArg...)
	args = append(args, []string{"--handle", handle, "--network", spec}...)

	return gardener.Hooks{
		Prestart: gardener.Hook{
			Path: p.path,
			Args: append([]string{p.path, "up"}, args...),
		},
		Poststop: gardener.Hook{
			Path: p.path,
			Args: append([]string{p.path, "down"}, args...),
		},
	}, nil
}

func (Plugin) Capacity() uint64 {
	return 0
}

func (Plugin) Destroy(log lager.Logger, handle string) error {
	return nil
}

func (Plugin) NetIn(handle string, hostPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, nil
}

func (Plugin) NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error {
	return nil
}

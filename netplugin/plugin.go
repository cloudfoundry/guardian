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

func (p Plugin) Hook(log lager.Logger, handle, spec string) (gardener.Hook, error) {
	args := []string{p.path}
	args = append(args, p.extraArg...)
	args = append(args, []string{"up", "--handle", handle, "--network", spec}...)

	return gardener.Hook{
		Path: p.path,
		Args: args,
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

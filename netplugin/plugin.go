package netplugin

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/pivotal-golang/lager"
)

const NetworkPropertyPrefix = "network."

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

func networkProperties(containerProperties garden.Properties) garden.Properties {
	properties := garden.Properties{}

	for k, value := range containerProperties {
		if strings.HasPrefix(k, NetworkPropertyPrefix) {
			key := strings.TrimPrefix(k, NetworkPropertyPrefix)
			properties[key] = value
		}
	}

	return properties
}

func (p Plugin) Hooks(log lager.Logger, containerSpec garden.ContainerSpec) (gardener.Hooks, error) {
	pathAndExtraArgs := append([]string{p.path}, p.extraArg...)

	propertiesJSON, err := json.Marshal(networkProperties(containerSpec.Properties))
	if err != nil {
		return gardener.Hooks{}, fmt.Errorf("marshaling network properties: %s", err) // not tested
	}

	networkPluginFlags := []string{
		"--handle", containerSpec.Handle,
		"--network", containerSpec.Network,
		"--properties", string(propertiesJSON),
	}

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

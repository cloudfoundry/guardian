package netplugin

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

const NetworkPropertyPrefix = "network."

type ExternalBinaryNetworker struct {
	commandRunner command_runner.CommandRunner
	path          string
	extraArg      []string
}

func New(commandRunner command_runner.CommandRunner, path string, extraArg ...string) kawasaki.Networker {
	return &ExternalBinaryNetworker{
		commandRunner: commandRunner,
		path:          path,
		extraArg:      extraArg,
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

func (p *ExternalBinaryNetworker) Network(log lager.Logger, containerSpec garden.ContainerSpec, pid int, bundlePath string) error {
	pathAndExtraArgs := append([]string{p.path}, p.extraArg...)
	propertiesJSON, err := json.Marshal(networkProperties(containerSpec.Properties))
	if err != nil {
		return fmt.Errorf("marshaling network properties: %s", err) // not tested
	}

	networkPluginFlags := []string{
		"--handle", containerSpec.Handle,
		"--network", containerSpec.Network,
		"--properties", string(propertiesJSON),
	}

	upArgs := append(pathAndExtraArgs, "--action", "up")
	upArgs = append(upArgs, networkPluginFlags...)

	cmd := exec.Command(p.path)
	cmd.Args = upArgs
	return p.commandRunner.Run(cmd)
}

func (p *ExternalBinaryNetworker) Destroy(log lager.Logger, handle string) error {
	pathAndExtraArgs := append([]string{p.path}, p.extraArg...)

	networkPluginFlags := []string{
		"--handle", handle,
	}

	downArgs := append(pathAndExtraArgs, "--action", "down")
	downArgs = append(downArgs, networkPluginFlags...)

	cmd := exec.Command(p.path)
	cmd.Args = downArgs
	return p.commandRunner.Run(cmd)
}

func (p *ExternalBinaryNetworker) Restore(log lager.Logger, handle string) error {
	return nil
}

func (p *ExternalBinaryNetworker) Capacity() (m uint64) {
	return math.MaxUint64
}

func (p *ExternalBinaryNetworker) NetIn(log lager.Logger, handle string, externalPort, containerPort uint32) (uint32, uint32, error) {
	return 0, 0, nil
}

func (p *ExternalBinaryNetworker) NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error {
	return nil
}

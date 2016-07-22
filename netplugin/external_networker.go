package netplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
)

const NetworkPropertyPrefix = "network."

type ExternalBinaryNetworker struct {
	commandRunner command_runner.CommandRunner
	configStore   kawasaki.ConfigStore
	path          string
	extraArg      []string
}

func New(commandRunner command_runner.CommandRunner, configStore kawasaki.ConfigStore, path string, extraArg ...string) kawasaki.Networker {
	return &ExternalBinaryNetworker{
		commandRunner: commandRunner,
		configStore:   configStore,
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

func (p *ExternalBinaryNetworker) Network(log lager.Logger, containerSpec garden.ContainerSpec, pid int) error {
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
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput

	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	_, err = io.WriteString(input, fmt.Sprintf("{\"PID\":%d}", pid))
	if err != nil {
		return err
	}
	input.Close()

	err = p.commandRunner.Run(cmd)
	if err != nil {
		return err
	}

	if len(cmdOutput.Bytes()) == 0 {
		return nil
	}

	var properties map[string]map[string]string

	if err := json.Unmarshal(cmdOutput.Bytes(), &properties); err != nil {
		return fmt.Errorf("network plugin returned invalid JSON: %s", err)
	}

	if _, ok := properties["properties"]; !ok {
		return fmt.Errorf("network plugin returned JSON without a properties key")
	}

	for k, v := range properties["properties"] {
		p.configStore.Set(containerSpec.Handle, k, v)
	}

	return nil
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

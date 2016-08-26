package netplugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
)

const NetworkPropertyPrefix = "network."
const NetOutKey = NetworkPropertyPrefix + "external-networker.net-out"

type externalBinaryNetworker struct {
	commandRunner    command_runner.CommandRunner
	configStore      kawasaki.ConfigStore
	portPool         kawasaki.PortPool
	externalIP       net.IP
	dnsServers       []net.IP
	resolvConfigurer kawasaki.DnsResolvConfigurer
	path             string
	extraArg         []string
}

func New(
	commandRunner command_runner.CommandRunner,
	configStore kawasaki.ConfigStore,
	portPool kawasaki.PortPool,
	externalIP net.IP,
	dnsServers []net.IP,
	resolvConfigurer kawasaki.DnsResolvConfigurer,
	path string,
	extraArg []string,
) ExternalNetworker {
	return &externalBinaryNetworker{
		commandRunner:    commandRunner,
		configStore:      configStore,
		portPool:         portPool,
		externalIP:       externalIP,
		dnsServers:       dnsServers,
		resolvConfigurer: resolvConfigurer,
		path:             path,
		extraArg:         extraArg,
	}
}

type ExternalNetworker interface {
	gardener.Networker
	gardener.Starter
}

func (p *externalBinaryNetworker) Start() error { return nil }

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

func (p *externalBinaryNetworker) Network(log lager.Logger, containerSpec garden.ContainerSpec, pid int) error {
	p.configStore.Set(containerSpec.Handle, gardener.ExternalIPKey, p.externalIP.String())

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
	cmdStderr := &bytes.Buffer{}
	cmd.Stderr = cmdStderr
	cmd.Stdin = strings.NewReader(fmt.Sprintf("{\"PID\":%d}", pid))

	err = p.commandRunner.Run(cmd)
	if err != nil {
		log.Error("external-networker-result", err, lager.Data{"output": cmdStderr.String()})
		return err
	}

	log.Info("external-networker-result", lager.Data{"output": cmdStderr.String()})

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

	containerIP, ok := p.configStore.Get(containerSpec.Handle, gardener.ContainerIPKey)
	if !ok {
		return fmt.Errorf("no container ip")
	}

	log.Info("external-binary-write-dns-to-config", lager.Data{
		"dnsServers": p.dnsServers,
	})
	cfg := kawasaki.NetworkConfig{
		ContainerIP:     net.ParseIP(containerIP),
		BridgeIP:        net.ParseIP(containerIP),
		ContainerHandle: containerSpec.Handle,
		DNSServers:      p.dnsServers,
	}

	err = p.resolvConfigurer.Configure(log, cfg, pid)
	if err != nil {
		return err
	}

	return nil
}

func (p *externalBinaryNetworker) Destroy(log lager.Logger, handle string) error {
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

func (p *externalBinaryNetworker) Restore(log lager.Logger, handle string) error {
	return nil
}

func (p *externalBinaryNetworker) Capacity() (m uint64) {
	return math.MaxUint64
}

func (p *externalBinaryNetworker) NetIn(log lager.Logger, handle string, externalPort, containerPort uint32) (uint32, uint32, error) {
	var err error
	if externalPort == 0 {
		externalPort, err = p.portPool.Acquire()
		if err != nil {
			return 0, 0, err
		}
	}

	if containerPort == 0 {
		containerPort = externalPort
	}

	if err := kawasaki.AddPortMapping(log, p.configStore, handle, garden.PortMapping{
		HostPort:      externalPort,
		ContainerPort: containerPort,
	}); err != nil {
		return 0, 0, err
	}
	return externalPort, containerPort, nil
}

func (p *externalBinaryNetworker) exec(log lager.Logger, action, handle, stdin string, cmdArgs ...string) ([]byte, error) {
	args := append([]string{p.path}, p.extraArg...)
	args = append(args, "--action", action, "--handle", handle)
	cmd := exec.Command(p.path)
	cmd.Args = append(args, cmdArgs...)
	stdout := &bytes.Buffer{}
	cmd.Stdout = stdout
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	cmd.Stdin = strings.NewReader(stdin)

	err := p.commandRunner.Run(cmd)
	logData := lager.Data{"stderr": stderr.String(), "stdout": stdout.String()}
	if err != nil {
		log.Error("external-networker-result", err, logData)
		return stdout.Bytes(), fmt.Errorf("external networker: %s", err)
	}
	log.Info("external-networker-result", logData)
	return stdout.Bytes(), nil
}

func (p *externalBinaryNetworker) NetOut(log lager.Logger, handle string, rule garden.NetOutRule) error {
	containerIP, ok := p.configStore.Get(handle, gardener.ContainerIPKey)
	if !ok {
		return fmt.Errorf("cannot find container [%s]\n", handle)
	}

	var props = struct {
		ContainerIP string            `json:"container_ip"`
		NetOutRule  garden.NetOutRule `json:"netout_rule"`
	}{
		ContainerIP: containerIP,
		NetOutRule:  rule,
	}
	propertiesJSON, err := json.Marshal(props)
	if err != nil {
		return fmt.Errorf("marshaling netout rule: %s", err) // not tested
	}

	_, err = p.exec(log, "net-out", handle, "", []string{"--properties", string(propertiesJSON)}...)
	if err != nil {
		return err
	}

	return nil
}

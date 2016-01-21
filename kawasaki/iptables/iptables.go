package iptables

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry/gunk/command_runner"
)

var protocols = map[garden.Protocol]string{
	garden.ProtocolAll:  "all",
	garden.ProtocolTCP:  "tcp",
	garden.ProtocolICMP: "icmp",
	garden.ProtocolUDP:  "udp",
}

type IPTables struct {
	runner                                                                                         command_runner.CommandRunner
	preroutingChain, postroutingChain, inputChain, forwardChain, defaultChain, instanceChainPrefix string
}

type Chains struct {
	Prerouting, Postrouting, Input, Forward, Default string
}

func New(runner command_runner.CommandRunner, chainPrefix string) *IPTables {
	return &IPTables{
		runner: runner,

		preroutingChain:     chainPrefix + "prerouting",
		postroutingChain:    chainPrefix + "postrouting",
		inputChain:          chainPrefix + "input",
		forwardChain:        chainPrefix + "forward",
		defaultChain:        chainPrefix + "default",
		instanceChainPrefix: chainPrefix + "instance-",
	}
}

type rule interface {
	flags(chain string) []string
}

type iptablesFlags []string

func (flags iptablesFlags) flags(chain string) []string {
	return flags
}

func (iptables *IPTables) run(action string, cmd *exec.Cmd) error {
	var buff bytes.Buffer
	cmd.Stderr = &buff

	if err := iptables.runner.Run(cmd); err != nil {
		return fmt.Errorf("iptables %s: %s", action, buff.String())
	}

	return nil
}

func (iptables *IPTables) instanceChain(instanceId string) string {
	return iptables.instanceChainPrefix + instanceId
}

func (iptables *IPTables) appendRule(chain string, rule rule) error {
	return iptables.run("append", exec.Command("/sbin/iptables", append([]string{"-w", "-A", chain}, rule.flags(chain)...)...))
}

func (iptables *IPTables) prependRule(chain string, rule rule) error {
	return iptables.run("prepend", exec.Command("/sbin/iptables", append([]string{"-w", "-I", chain, "1"}, rule.flags(chain)...)...))
}

func natRule(destination string, destinationPort uint32, containerIP string, containerPort uint32) rule {
	return iptablesFlags([]string{
		"--table", "nat",
		"--protocol", "tcp",
		"--destination", destination,
		"--destination-port", fmt.Sprintf("%d", destinationPort),
		"--jump", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
	})
}

func rejectRule(destination string) rule {
	return iptablesFlags([]string{
		"--destination", destination,
		"--jump", "REJECT",
	})
}

type singleFilterRule struct {
	Protocol garden.Protocol
	Networks *garden.IPRange
	Ports    *garden.PortRange
	ICMPs    *garden.ICMPControl
	Log      bool
}

func (r singleFilterRule) flags(chain string) (params []string) {
	protocolString := protocols[r.Protocol]

	params = append(params, "--protocol", protocolString)

	network := r.Networks
	if network != nil {
		if network.Start != nil && network.End != nil {
			params = append(params, "-m", "iprange", "--dst-range", network.Start.String()+"-"+network.End.String())
		} else if network.Start != nil {
			params = append(params, "--destination", network.Start.String())
		} else if network.End != nil {
			params = append(params, "--destination", network.End.String())
		}
	}

	ports := r.Ports
	if ports != nil {
		if ports.End != ports.Start {
			params = append(params, "--destination-port", fmt.Sprintf("%d:%d", ports.Start, ports.End))
		} else {
			params = append(params, "--destination-port", fmt.Sprintf("%d", ports.Start))
		}
	}

	if r.ICMPs != nil {
		icmpType := fmt.Sprintf("%d", r.ICMPs.Type)
		if r.ICMPs.Code != nil {
			icmpType = fmt.Sprintf("%d/%d", r.ICMPs.Type, *r.ICMPs.Code)
		}

		params = append(params, "--icmp-type", icmpType)
	}

	if r.Log {
		params = append(params, "--goto", chain+"-log")
	} else {
		params = append(params, "--jump", "RETURN")
	}

	return params
}

package iptables

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry/gunk/command_runner"
)

type PortForwarder struct {
	runner command_runner.CommandRunner
}

func NewPortForwarder(runner command_runner.CommandRunner) *PortForwarder {
	return &PortForwarder{
		runner: runner,
	}
}

func (p *PortForwarder) Forward(spec *kawasaki.PortForwarderSpec) error {
	buff := bytes.NewBufferString("")

	cmd := exec.Command("iptables", "--wait", "--table", "nat",
		"-A", spec.IPTableChain,
		"--protocol", "tcp",
		"--destination", spec.ExternalIP.String(),
		"--destination-port", fmt.Sprintf("%d", spec.FromPort),
		"--jump", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", spec.ContainerIP.String(), spec.ToPort))

	cmd.Stderr = buff
	err := p.runner.Run(cmd)
	if err != nil {
		return fmt.Errorf("PortForwarder: %s", buff.String())
	}
	return nil
}

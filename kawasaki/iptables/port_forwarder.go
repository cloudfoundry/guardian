package iptables

import "code.cloudfoundry.org/guardian/kawasaki"

type PortForwarder struct {
	iptables *IPTablesController
}

func NewPortForwarder(iptables *IPTablesController) *PortForwarder {
	return &PortForwarder{
		iptables: iptables,
	}
}

func (p *PortForwarder) Forward(spec kawasaki.PortForwarderSpec) error {
	return p.iptables.appendRule(
		p.iptables.InstanceChain(spec.InstanceID),
		natRule(
			spec.ExternalIP.String(),
			spec.FromPort,
			spec.ContainerIP.String(),
			spec.ToPort,
			spec.Handle,
		),
	)
}

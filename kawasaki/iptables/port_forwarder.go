package iptables

import "github.com/cloudfoundry-incubator/guardian/kawasaki"

type PortForwarder struct {
	iptables *IPTables
}

func NewPortForwarder(iptables *IPTables) *PortForwarder {
	return &PortForwarder{
		iptables: iptables,
	}
}

func (p *PortForwarder) Forward(spec kawasaki.PortForwarderSpec) error {
	return p.iptables.appendRule(
		p.iptables.instanceChain(spec.InstanceID),
		natRule(
			spec.ExternalIP.String(),
			spec.FromPort,
			spec.ContainerIP.String(),
			spec.ToPort,
		),
	)
}

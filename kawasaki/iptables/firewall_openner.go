package iptables

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

type FirewallOpener struct {
	iptables IPTables
}

func NewFirewallOpener(iptables IPTables) *FirewallOpener {
	return &FirewallOpener{
		iptables: iptables,
	}
}

func (f *FirewallOpener) Open(logger lager.Logger, instance string, r garden.NetOutRule) error {
	chain := f.iptables.InstanceChain(instance)

	logger = logger.Session("prepend-filter-rule", lager.Data{"rule": r, "instance": instance, "chain": chain})
	logger.Debug("started")

	if len(r.Ports) > 0 && !allowsPort(r.Protocol) {
		return fmt.Errorf("Ports cannot be specified for Protocol %s", strings.ToUpper(protocols[r.Protocol]))
	}

	if _, ok := protocols[r.Protocol]; !ok {
		return fmt.Errorf("invalid protocol: %d", r.Protocol)
	}

	filter := SingleFilterRule{
		Protocol: r.Protocol,
		ICMPs:    r.ICMPs,
		Log:      r.Log,
	}

	// It should still loop once even if there are no networks or ports.
	for i := 0; i < len(r.Ports) || i == 0; i++ {
		for j := 0; j < len(r.Networks) || j == 0; j++ {
			// Preserve nils unless there are ports specified
			if len(r.Ports) > 0 {
				filter.Ports = &r.Ports[i]
			}

			// Preserve nils unless there are networks specified
			if len(r.Networks) > 0 {
				filter.Networks = &r.Networks[j]
			}

			if err := f.iptables.PrependRule(chain, filter); err != nil {
				return err
			}
		}
	}

	logger.Debug("ending")
	return nil
}

func allowsPort(p garden.Protocol) bool {
	return p == garden.ProtocolTCP || p == garden.ProtocolUDP
}

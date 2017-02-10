package iptables

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/garden"
)

var protocols = map[garden.Protocol]string{
	garden.ProtocolAll:  "all",
	garden.ProtocolTCP:  "tcp",
	garden.ProtocolICMP: "icmp",
	garden.ProtocolUDP:  "udp",
}

type ruleTranslator struct {
}

func NewRuleTranslator() RuleTranslator {
	return &ruleTranslator{}
}

func (t *ruleTranslator) TranslateRule(handle string, gardenRule garden.NetOutRule) ([]Rule, error) {
	if len(gardenRule.Ports) > 0 && !allowsPort(gardenRule.Protocol) {
		return nil, fmt.Errorf("Ports cannot be specified for Protocol %s", strings.ToUpper(protocols[gardenRule.Protocol]))
	}

	if _, ok := protocols[gardenRule.Protocol]; !ok {
		return nil, fmt.Errorf("invalid protocol: %d", gardenRule.Protocol)
	}

	iptablesRule := SingleFilterRule{
		Protocol: gardenRule.Protocol,
		ICMPs:    gardenRule.ICMPs,
		Log:      gardenRule.Log,
		Handle:   handle,
	}

	iptablesRules := []Rule{}
	// It should still loop once even if there are no networks or ports.
	for i := 0; i < len(gardenRule.Ports) || i == 0; i++ {
		for j := 0; j < len(gardenRule.Networks) || j == 0; j++ {
			// Preserve nils unless there are ports specified
			if len(gardenRule.Ports) > 0 {
				iptablesRule.Ports = &gardenRule.Ports[i]
			}

			// Preserve nils unless there are networks specified
			if len(gardenRule.Networks) > 0 {
				iptablesRule.Networks = &gardenRule.Networks[j]
			}

			iptablesRules = append(iptablesRules, iptablesRule)
		}
	}

	return iptablesRules, nil
}

func allowsPort(p garden.Protocol) bool {
	return p == garden.ProtocolTCP || p == garden.ProtocolUDP
}

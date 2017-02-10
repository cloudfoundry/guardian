package iptables

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . RuleTranslator
type RuleTranslator interface {
	TranslateRule(handle string, gardenRule garden.NetOutRule) ([]Rule, error)
}

type FirewallOpener struct {
	ruleTranslator RuleTranslator
	iptables       IPTables
}

func NewFirewallOpener(ruleTranslator RuleTranslator, iptables IPTables) *FirewallOpener {
	return &FirewallOpener{
		ruleTranslator: ruleTranslator,
		iptables:       iptables,
	}
}

func (f *FirewallOpener) Open(logger lager.Logger, instance, handle string, rule garden.NetOutRule) error {
	chain := f.iptables.InstanceChain(instance)
	logger = logger.Session("prepend-filter-rule", lager.Data{
		"rule":     rule,
		"instance": instance,
		"chain":    chain,
	})
	logger.Debug("started")
	defer logger.Debug("ending")

	iptableRules, err := f.ruleTranslator.TranslateRule(handle, rule)
	if err != nil {
		return err
	}

	for _, iptableRules := range iptableRules {
		if err := f.iptables.PrependRule(chain, iptableRules); err != nil {
			return err
		}
	}

	return nil
}

func (f *FirewallOpener) BulkOpen(logger lager.Logger, instance, handle string, rules []garden.NetOutRule) error {
	chain := f.iptables.InstanceChain(instance)
	logger = logger.Session("prepend-filter-rule", lager.Data{
		"rules":    rules,
		"instance": instance,
		"chain":    chain,
	})
	logger.Debug("started")
	defer logger.Debug("ending")

	collatedIPTablesRules := []Rule{}
	for _, rule := range rules {
		iptablesRules, err := f.ruleTranslator.TranslateRule(handle, rule)
		if err != nil {
			return err
		}

		collatedIPTablesRules = append(collatedIPTablesRules, iptablesRules...)
	}

	return f.iptables.BulkPrependRules(chain, collatedIPTablesRules)
}

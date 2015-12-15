package iptables

import (
	"net"

	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Chain
type Chain interface {
	Setup(instanceChain, bridgeName string, ip net.IP, network *net.IPNet) error
	Teardown(instanceChain string) error
}

type Manager struct {
	Chains []Chain
	*Starter
}

func NewManager(fc FilterConfig, nc NATConfig, chainPrefix, nicPrefix string, runner command_runner.CommandRunner, logger lager.Logger) *Manager {
	return &Manager{
		Chains: []Chain{
			NewFilterChain(fc, runner, logger),
			NewNATChain(nc, runner, logger),
		},
		Starter: &Starter{
			runner:      runner,
			fc:          fc,
			nc:          nc,
			chainPrefix: chainPrefix,
			nicPrefix:   nicPrefix,
		},
	}
}

func (mgr *Manager) Apply(instanceChain, bridgeName string, ip net.IP, network *net.IPNet) error {
	if err := mgr.Destroy(instanceChain); err != nil {
		return err
	}

	for index, chain := range mgr.Chains {
		if err := chain.Setup(instanceChain, bridgeName, ip, network); err != nil {
			for i := 0; i < index; i++ {
				mgr.Chains[i].Teardown(instanceChain)
			}
			return err
		}

	}

	return nil
}

func (mgr *Manager) Destroy(instanceChain string) error {
	var lastErr error
	for _, chain := range mgr.Chains {
		if err := chain.Teardown(instanceChain); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

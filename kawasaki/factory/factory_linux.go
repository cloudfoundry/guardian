package factory

import (
	"os"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/configure"
	"code.cloudfoundry.org/guardian/kawasaki/devices"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
)

func NewDefaultConfigurer(ipt *iptables.IPTablesController) kawasaki.Configurer {
	hostConfigurer := &configure.Host{
		Veth:   &devices.VethCreator{},
		Link:   &devices.Link{},
		Bridge: &devices.Bridge{},
	}

	containerCfgApplier := &configure.Container{
		Link: &devices.Link{},
	}

	return kawasaki.NewConfigurer(
		&kawasaki.ResolvFactory{},
		hostConfigurer,
		containerCfgApplier,
		iptables.NewInstanceChainCreator(ipt),
		os.Open,
		&netns.Execer{},
	)
}

package factory

import (
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/configure"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/devices"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/netns"
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
		&netns.Execer{},
	)
}

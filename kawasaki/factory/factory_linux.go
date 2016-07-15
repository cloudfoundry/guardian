package factory

import (
	"os"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/configure"
	"code.cloudfoundry.org/guardian/kawasaki/devices"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
)

func NewDefaultConfigurer(ipt *iptables.IPTablesController) kawasaki.Configurer {
	hostConfigurer := &configure.Host{
		Veth:   &devices.VethCreator{},
		Link:   &devices.Link{},
		Bridge: &devices.Bridge{},
	}

	containerConfigurer := &configure.Container{}

	return kawasaki.NewConfigurer(
		&kawasaki.ResolvFactory{},
		hostConfigurer,
		containerConfigurer,
		iptables.NewInstanceChainCreator(ipt),
		os.Open,
	)
}

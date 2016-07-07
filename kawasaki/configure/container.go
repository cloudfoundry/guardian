package configure

import (
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/pivotal-golang/lager"
)

type Container struct {
	Link interface {
		AddIP(intf *net.Interface, ip net.IP, subnet *net.IPNet) error
		AddDefaultGW(intf *net.Interface, ip net.IP) error
		SetUp(intf *net.Interface) error
		SetMTU(intf *net.Interface, mtu int) error
		InterfaceByName(name string) (*net.Interface, bool, error)
	}
}

func (c *Container) Apply(log lager.Logger, config kawasaki.NetworkConfig) error {
	return c.configureContainerIntf(
		log,
		config.ContainerIntf,
		config.ContainerIP,
		config.BridgeIP,
		config.Subnet,
		config.Mtu,
	)
}

func (c *Container) configureContainerIntf(log lager.Logger, name string, ip, gatewayIP net.IP, subnet *net.IPNet, mtu int) (err error) {
	cLog := log.Session("configure-container", lager.Data{
		"name":    name,
		"ip":      ip,
		"gateway": gatewayIP,
		"subnet":  subnet,
		"mtu":     mtu,
	})

	cLog.Debug("start")

	var found bool
	var intf *net.Interface
	if intf, found, err = c.Link.InterfaceByName(name); !found || err != nil {
		return &FindLinkError{err, "container", name}
	}

	if err := c.Link.AddIP(intf, ip, subnet); err != nil {
		return &ConfigureLinkError{err, "container", intf, ip, subnet}
	}

	if err := c.Link.SetUp(intf); err != nil {
		return &LinkUpError{err, intf, "container"}
	}

	if err := c.Link.AddDefaultGW(intf, gatewayIP); err != nil {
		return &ConfigureDefaultGWError{err, intf, gatewayIP}
	}

	if err := c.Link.SetMTU(intf, mtu); err != nil {
		return &MTUError{err, intf, mtu}
	}

	cLog.Debug("done")
	return nil
}

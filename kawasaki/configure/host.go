package configure

import (
	"fmt"
	"net"
	"os"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/lager"
)

type Host struct {
	Veth interface {
		Create(hostIfcName, containerIfcName string) (*net.Interface, *net.Interface, error)
	}

	Link interface {
		SetUp(intf *net.Interface) error
		SetMTU(intf *net.Interface, mtu int) error
		SetNs(intf *net.Interface, fd int) error
		InterfaceByName(name string) (*net.Interface, bool, error)
	}

	Bridge interface {
		Create(bridgeName string, ip net.IP, subnet *net.IPNet) (*net.Interface, error)
		Add(bridge, slave *net.Interface) error
		Destroy(bridgeName string) error
	}

	FileOpener interface {
		Open(path string) (*os.File, error)
	}
}

func (c *Host) Apply(logger lager.Logger, config kawasaki.NetworkConfig, pid int) error {
	var (
		err       error
		host      *net.Interface
		container *net.Interface
		bridge    *net.Interface
	)

	cLog := logger.Session("configure-host", lager.Data{
		"bridgeName":     config.BridgeName,
		"bridgeIP":       config.BridgeIP,
		"subnet":         config.Subnet,
		"containerIface": config.ContainerIntf,
		"hostIface":      config.HostIntf,
		"mtu":            config.Mtu,
		"pid":            pid,
	})

	cLog.Debug("configuring")

	if bridge, err = c.configureBridgeIntf(cLog, config.BridgeName, config.BridgeIP, config.Subnet); err != nil {
		return err
	}

	if host, container, err = c.configureVethPair(cLog, config.HostIntf, config.ContainerIntf); err != nil {
		return err
	}

	if err = c.configureHostIntf(cLog, host, bridge, config.Mtu); err != nil {
		return err
	}

	netns, err := c.FileOpener.Open(fmt.Sprintf("/proc/%d/ns/net", pid))
	if err != nil {
		return err
	}
	defer netns.Close()

	// move container end in to container
	if err = c.Link.SetNs(container, int(netns.Fd())); err != nil {
		return &SetNsFailedError{err, container, netns}
	}

	return nil
}

func (c *Host) Destroy(config kawasaki.NetworkConfig) error {
	return c.Bridge.Destroy(config.BridgeName)
}

func (c *Host) configureBridgeIntf(log lager.Logger, name string, ip net.IP, subnet *net.IPNet) (*net.Interface, error) {
	log = log.Session("bridge-interface")

	log.Debug("find")
	bridge, bridgeExists, err := c.Link.InterfaceByName(name)
	if err != nil || !bridgeExists {
		bridge, err = c.Bridge.Create(name, ip, subnet)
		if err != nil {
			log.Error("create", err)
			return nil, err
		}
	}

	log.Debug("bring-up")
	if err = c.Link.SetUp(bridge); err != nil {
		log.Error("bring-up", err)
		return nil, &LinkUpError{err, bridge, "bridge"}
	}

	return bridge, nil
}

func (c *Host) configureVethPair(log lager.Logger, hostName, containerName string) (*net.Interface, *net.Interface, error) {
	log = log.Session("veth")

	log.Debug("create")
	if host, container, err := c.Veth.Create(hostName, containerName); err != nil {
		log.Error("create", err)
		return nil, nil, &VethPairCreationError{err, hostName, containerName}
	} else {
		return host, container, err
	}
}

func (c *Host) configureHostIntf(log lager.Logger, intf *net.Interface, bridge *net.Interface, mtu int) error {
	log = log.Session("host-interface", lager.Data{
		"bridge-interface": bridge,
		"host-interface":   intf,
	})

	log.Debug("set-mtu")
	if err := c.Link.SetMTU(intf, mtu); err != nil {
		log.Error("set-mtu", err)
		return &MTUError{err, intf, mtu}
	}

	log.Debug("add-to-bridge")
	if err := c.Bridge.Add(bridge, intf); err != nil {
		log.Error("add-to-bridge", err)
		return &AddToBridgeError{err, bridge, intf}
	}

	log.Debug("bring-link-up")
	if err := c.Link.SetUp(intf); err != nil {
		log.Error("bring-link-up", err)
		return &LinkUpError{err, intf, "host"}
	}

	return nil
}

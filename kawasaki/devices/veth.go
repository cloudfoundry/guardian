package devices

import (
	"fmt"
	"net"

	"github.com/docker/libcontainer/netlink"
)

type VethCreator struct{}

func (VethCreator) Create(hostIfcName, containerIfcName string) (host, container *net.Interface, err error) {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	if err := netlink.NetworkCreateVethPair(hostIfcName, containerIfcName, 1); err != nil {
		return nil, nil, fmt.Errorf("devices: create veth pair: %v", err)
	}

	if host, err = net.InterfaceByName(hostIfcName); err != nil {
		return nil, nil, fmt.Errorf("devices: look up created host interface: %v", err)
	}

	if container, err = net.InterfaceByName(containerIfcName); err != nil {
		return nil, nil, fmt.Errorf("devices: look up created container interface: %v", err)
	}

	return host, container, nil
}

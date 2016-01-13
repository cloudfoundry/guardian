package devices

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

type VethCreator struct{}

func (VethCreator) Create(hostIfcName, containerIfcName string) (host, container *net.Interface, err error) {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: hostIfcName, TxQLen: 1},
		PeerName:  containerIfcName,
	}

	if err := netlink.LinkAdd(veth); err != nil {
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

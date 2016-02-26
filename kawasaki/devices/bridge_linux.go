package devices

import (
	"fmt"
	"net"
	"sync"

	"github.com/vishvananda/netlink"
)

// netlink is not thread-safe, all calls to netlink should be guarded by this mutex
var netlinkMu *sync.Mutex = new(sync.Mutex)

type Bridge struct{}

// Create creates a bridge device and returns the interface.
// If the device already exists, returns the existing interface.
func (Bridge) Create(name string, ip net.IP, subnet *net.IPNet) (intf *net.Interface, err error) {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	link := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: name}}

	if err := netlink.LinkAdd(link); err != nil && err.Error() != "file exists" {
		return nil, fmt.Errorf("devices: create bridge: %v", err)
	}

	if intf, err = net.InterfaceByName(name); err != nil {
		return nil, fmt.Errorf("devices: look up created bridge interface: %v", err)
	}

	addr := &netlink.Addr{IPNet: &net.IPNet{IP: ip, Mask: subnet.Mask}}
	if err = netlink.AddrAdd(link, addr); err != nil && err.Error() != "file exists" {
		return nil, fmt.Errorf("devices: add IP to bridge: %v", err)
	}
	return intf, nil
}

func (Bridge) Add(bridge, slaveIf *net.Interface) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	master, err := netlink.LinkByName(bridge.Name)
	if err != nil {
		return err
	}

	slave, err := netlink.LinkByName(slaveIf.Name)
	if err != nil {
		return err
	}
	return netlink.LinkSetMaster(slave, master.(*netlink.Bridge))
}

func (Bridge) Destroy(bridge string) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	intfs, err := net.Interfaces()
	if err != nil {
		return err
	}

	for _, i := range intfs {
		if i.Name == bridge {
			link := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: bridge}}
			return netlink.LinkDel(link)
		}
	}

	return nil
}

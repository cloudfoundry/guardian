package devices

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

// netlink is not thread-safe, all calls to netlink should be guarded by this mutex
var netlinkMu *sync.Mutex = new(sync.Mutex)

type Bridge struct{}

// Create creates a bridge device and returns the interface.
// If the device already exists, returns the existing interface.
func (Bridge) Create(name string, ip net.IP, subnet *net.IPNet) (intf *net.Interface, err error) {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	if intf, _ := net.InterfaceByName(name); intf != nil {
		return intf, nil
	}

	link := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: name}}
	if err := netlink.LinkAdd(link); err != nil && err.Error() != "file exists" {
		return nil, fmt.Errorf("devices: create bridge: %v", err)
	}

	if err := netlink.BridgeSetMcastSnoop(link, false); err != nil {
		return nil, fmt.Errorf("devices: disable multicast snooping: %v", err)
	}

	hAddr, _ := net.ParseMAC(randMacAddr())
	err = netlink.LinkSetHardwareAddr(link, hAddr)
	if err != nil {
		return nil, fmt.Errorf("devices: set hardware address: %v", err)
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

func randMacAddr() string {
	hw := make(net.HardwareAddr, 6)
	for i := 0; i < 6; i++ {
		hw[i] = byte(rnd.Intn(255))
	}
	hw[0] &^= 0x1 // clear multicast bit
	hw[0] |= 0x2  // set local assignment bit (IEEE802)
	return hw.String()
}

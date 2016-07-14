package devices

import (
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"code.cloudfoundry.org/garden"
	"github.com/vishvananda/netlink"
)

type Link struct {
}

func (Link) AddIP(intf *net.Interface, ip net.IP, subnet *net.IPNet) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	link, err := netlink.LinkByName(intf.Name)
	if err != nil {
		return errF(err)
	}

	addr := &netlink.Addr{IPNet: &net.IPNet{IP: ip, Mask: subnet.Mask}}
	return errF(netlink.AddrAdd(link, addr))
}

func (Link) AddDefaultGW(intf *net.Interface, ip net.IP) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	link, err := netlink.LinkByName(intf.Name)
	if err != nil {
		return errF(err)
	}

	route := &netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: link.Attrs().Index,
		Gw:        ip,
	}

	return errF(netlink.RouteAdd(route))
}

func (Link) SetUp(intf *net.Interface) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	link, err := netlink.LinkByName(intf.Name)
	if err != nil {
		return errF(err)
	}

	return errF(netlink.LinkSetUp(link))
}

func (Link) SetMTU(intf *net.Interface, mtu int) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	link, err := netlink.LinkByName(intf.Name)
	if err != nil {
		return errF(err)
	}

	return errF(netlink.LinkSetMTU(link, mtu))
}

func (Link) SetNs(intf *net.Interface, ns int) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	link, err := netlink.LinkByName(intf.Name)
	if err != nil {
		return errF(err)
	}

	return errF(netlink.LinkSetNsFd(link, ns))
}

func (Link) InterfaceByName(name string) (*net.Interface, bool, error) {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	intfs, err := net.Interfaces()
	if err != nil {
		return nil, false, errF(err)
	}

	for _, intf := range intfs {
		if intf.Name == name {
			return &intf, true, nil
		}
	}

	return nil, false, nil
}

func (Link) List() (names []string, err error) {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	intfs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range intfs {
		names = append(names, i.Name)
	}

	return names, nil
}

func (l Link) Statistics(name string) (stats garden.ContainerNetworkStat, err error) {
	var RxBytes, TxBytes uint64

	if RxBytes, err = intfStat(name, "rx_bytes"); err != nil {
		return stats, err
	}

	if TxBytes, err = intfStat(name, "tx_bytes"); err != nil {
		return stats, err
	}

	return garden.ContainerNetworkStat{
		RxBytes: RxBytes,
		TxBytes: TxBytes,
	}, nil
}

func intfStat(intf, statFile string) (stat uint64, err error) {
	data, err := ioutil.ReadFile(filepath.Join("/sys/class/net", intf, "statistics", statFile))
	if err != nil {
		return 0, err
	}

	stat, err = strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}

	return stat, nil
}

func errF(err error) error {
	if err == nil {
		return err
	}

	return fmt.Errorf("devices: %v", err)
}

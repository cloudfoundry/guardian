package devices

import (
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/docker/libcontainer/netlink"
)

type Link struct {
	Name string
}

func (Link) AddIP(intf *net.Interface, ip net.IP, subnet *net.IPNet) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	return errF(netlink.NetworkLinkAddIp(intf, ip, subnet))
}

func (Link) AddDefaultGW(intf *net.Interface, ip net.IP) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	return errF(netlink.AddDefaultGw(ip.String(), intf.Name))
}

func (Link) SetUp(intf *net.Interface) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	return errF(netlink.NetworkLinkUp(intf))
}

func (Link) SetMTU(intf *net.Interface, mtu int) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	return errF(netlink.NetworkSetMTU(intf, mtu))
}

func (Link) SetNs(intf *net.Interface, ns int) error {
	netlinkMu.Lock()
	defer netlinkMu.Unlock()

	return errF(netlink.NetworkSetNsFd(intf, ns))
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

func (l Link) Statistics() (stats garden.ContainerNetworkStat, err error) {
	var RxBytes, TxBytes uint64

	if RxBytes, err = intfStat(l.Name, "rx_bytes"); err != nil {
		return stats, err
	}

	if TxBytes, err = intfStat(l.Name, "tx_bytes"); err != nil {
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

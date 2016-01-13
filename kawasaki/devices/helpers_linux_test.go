package devices_test

import (
	"net"

	"github.com/vishvananda/netlink"
)

func cleanup(intfName string) error {
	if _, err := net.InterfaceByName(intfName); err == nil {
		link, err := netlink.LinkByName(intfName)
		if err != nil {
			return err
		}
		return netlink.LinkDel(link)
	}
	return nil
}

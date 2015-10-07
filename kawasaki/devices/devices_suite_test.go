package devices_test

import (
	"net"

	"github.com/docker/libcontainer/netlink"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestDevices(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Devices Suite")
}

func cleanup(intfName string) error {
	if _, err := net.InterfaceByName(intfName); err == nil {
		return netlink.NetworkLinkDel(intfName)
	}
	return nil
}

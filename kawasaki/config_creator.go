package kawasaki

import (
	"fmt"
	"net"
	"strings"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
)

const maxHandleBeforeTruncation = 8

type NetworkConfig struct {
	HostIntf      string
	ContainerIntf string
	BridgeName    string
	BridgeIP      net.IP
	ContainerIP   net.IP
	Subnet        *net.IPNet
	Mtu           int
}

type Creator struct{}

func NewConfigCreator() *Creator {
	return &Creator{}
}

func (c *Creator) Create(handle string, subnet *net.IPNet, ip net.IP) (NetworkConfig, error) {
	return NetworkConfig{
		HostIntf:      fmt.Sprintf("w-%s-0", truncate(handle)),
		ContainerIntf: fmt.Sprintf("w-%s-1", truncate(handle)),
		BridgeName:    fmt.Sprintf("br-%s", strings.Replace(subnet.IP.String(), ".", "-", -1)),
		ContainerIP:   ip,
		BridgeIP:      subnets.GatewayIP(subnet),
		Subnet:        subnet,
		Mtu:           1500,
	}, nil
}

func truncate(handle string) string {
	if len(handle) > maxHandleBeforeTruncation {
		return handle[:maxHandleBeforeTruncation]
	}

	return handle
}

package kawasaki

import (
	"fmt"
	"net"
	"strings"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/pivotal-golang/lager"
)

const maxHandleBeforeTruncation = 8

type NetworkConfig struct {
	HostIntf      string
	ContainerIntf string
	IPTableChain  string
	BridgeName    string
	BridgeIP      net.IP
	ContainerIP   net.IP
	Subnet        *net.IPNet
	Mtu           int
}

type Creator struct {
	interfacePrefix string
	chainPrefix     string
}

func NewConfigCreator(interfacePrefix, chainPrefix string) *Creator {
	return &Creator{
		interfacePrefix: interfacePrefix,
		chainPrefix:     chainPrefix,
	}
}

func (c *Creator) Create(log lager.Logger, handle string, subnet *net.IPNet, ip net.IP) (NetworkConfig, error) {
	return NetworkConfig{
		HostIntf:      fmt.Sprintf("%s-%s-0", c.interfacePrefix, truncate(handle)),
		ContainerIntf: fmt.Sprintf("%s-%s-1", c.interfacePrefix, truncate(handle)),
		BridgeName:    fmt.Sprintf("br-%s", strings.Replace(subnet.IP.String(), ".", "-", -1)),
		IPTableChain:  fmt.Sprintf("%s-%s", c.chainPrefix, truncate(handle)),
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

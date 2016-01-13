package kawasaki

import (
	"fmt"
	"net"
	"strings"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/pivotal-golang/lager"
)

const (
	maxInterfacePrefixLen = 2
	maxChainPrefixLen     = 16
)

//go:generate counterfeiter . IDGenerator

type IDGenerator interface {
	Generate() string
}

type NetworkConfig struct {
	HostIntf      string
	ContainerIntf string
	IPTableChain  string
	BridgeName    string
	BridgeIP      net.IP
	ContainerIP   net.IP
	ExternalIP    net.IP
	Subnet        *net.IPNet
	Mtu           int
}

type Creator struct {
	idGenerator     IDGenerator
	interfacePrefix string
	chainPrefix     string
	externalIP      net.IP
}

func NewConfigCreator(idGenerator IDGenerator, interfacePrefix, chainPrefix string, externalIP net.IP) *Creator {
	if len(interfacePrefix) > maxInterfacePrefixLen {
		panic("interface prefix is too long")
	}

	if len(chainPrefix) > maxChainPrefixLen {
		panic("chain prefix is too long")
	}

	return &Creator{
		idGenerator:     idGenerator,
		interfacePrefix: interfacePrefix,
		chainPrefix:     chainPrefix,
		externalIP:      externalIP,
	}
}

func (c *Creator) Create(log lager.Logger, handle string, subnet *net.IPNet, ip net.IP) (NetworkConfig, error) {
	id := c.idGenerator.Generate()
	return NetworkConfig{
		HostIntf:      fmt.Sprintf("%s%s-0", c.interfacePrefix, id),
		ContainerIntf: fmt.Sprintf("%s%s-1", c.interfacePrefix, id),
		BridgeName:    fmt.Sprintf("%s%s", c.interfacePrefix, strings.Replace(subnet.IP.String(), ".", "-", -1)),
		IPTableChain:  fmt.Sprintf("%s-%s", c.chainPrefix, id),
		ContainerIP:   ip,
		BridgeIP:      subnets.GatewayIP(subnet),
		ExternalIP:    c.externalIP,
		Subnet:        subnet,
		Mtu:           1500,
	}, nil
}

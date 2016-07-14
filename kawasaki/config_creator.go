package kawasaki

import (
	"encoding/hex"
	"fmt"
	"net"

	"code.cloudfoundry.org/guardian/kawasaki/subnets"
	"code.cloudfoundry.org/lager"
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
	ContainerHandle string
	HostIntf        string
	ContainerIntf   string
	IPTablePrefix   string
	IPTableInstance string
	BridgeName      string
	BridgeIP        net.IP
	ContainerIP     net.IP
	ExternalIP      net.IP
	Subnet          *net.IPNet
	Mtu             int
	DNSServers      []net.IP
}

type Creator struct {
	idGenerator     IDGenerator
	interfacePrefix string
	chainPrefix     string
	externalIP      net.IP
	dnsServers      []net.IP
	mtu             int
}

func NewConfigCreator(idGenerator IDGenerator, interfacePrefix, chainPrefix string, externalIP net.IP, dnsServers []net.IP, mtu int) *Creator {
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
		dnsServers:      dnsServers,
		mtu:             mtu,
	}
}

func (c *Creator) Create(log lager.Logger, handle string, subnet *net.IPNet, ip net.IP) (NetworkConfig, error) {
	id := c.idGenerator.Generate()
	return NetworkConfig{
		ContainerHandle: handle,
		HostIntf:        fmt.Sprintf("%s%s-0", c.interfacePrefix, id),
		ContainerIntf:   fmt.Sprintf("%s%s-1", c.interfacePrefix, id),

		BridgeName: fmt.Sprintf("%s%s%s", c.interfacePrefix, "brdg-", hex.EncodeToString(subnet.IP)),

		IPTablePrefix:   c.chainPrefix,
		IPTableInstance: id,
		ContainerIP:     ip,
		BridgeIP:        subnets.GatewayIP(subnet),
		ExternalIP:      c.externalIP,
		Subnet:          subnet,
		Mtu:             c.mtu,
		DNSServers:      c.dnsServers,
	}, nil
}

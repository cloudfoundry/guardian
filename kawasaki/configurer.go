package kawasaki

import (
	"net"
	"os"

	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . NetnsExecer
type NetnsExecer interface {
	Exec(netnsFD *os.File, cb func() error) error
}

type configurer struct {
	dnsResolvConfigurer  DnsResolvConfigurer
	hostConfigurer       HostConfigurer
	containerConfigurer  ContainerConfigurer
	instanceChainCreator InstanceChainCreator
}

//counterfeiter:generate . HostConfigurer
type HostConfigurer interface {
	Apply(logger lager.Logger, cfg NetworkConfig, pid int) error
	Destroy(cfg NetworkConfig) error
}

//counterfeiter:generate . InstanceChainCreator
type InstanceChainCreator interface {
	Create(logger lager.Logger, handle, instanceChain, bridgeName string, ip net.IP, network *net.IPNet) error
	Destroy(logger lager.Logger, instanceChain string) error
}

//counterfeiter:generate . ContainerConfigurer
type ContainerConfigurer interface {
	Apply(logger lager.Logger, cfg NetworkConfig, pid int) error
}

//counterfeiter:generate . DnsResolvConfigurer
type DnsResolvConfigurer interface {
	Configure(log lager.Logger, cfg NetworkConfig, pid int) error
}

func NewConfigurer(resolvConfigurer DnsResolvConfigurer, hostConfigurer HostConfigurer, containerConfigurer ContainerConfigurer, instanceChainCreator InstanceChainCreator) *configurer {
	return &configurer{
		dnsResolvConfigurer:  resolvConfigurer,
		hostConfigurer:       hostConfigurer,
		containerConfigurer:  containerConfigurer,
		instanceChainCreator: instanceChainCreator,
	}
}

func (c *configurer) Apply(log lager.Logger, cfg NetworkConfig, pid int) error {
	if err := c.dnsResolvConfigurer.Configure(log, cfg, pid); err != nil {
		return err
	}

	if err := c.hostConfigurer.Apply(log, cfg, pid); err != nil {
		return err
	}

	if err := c.instanceChainCreator.Create(log, cfg.ContainerHandle, cfg.IPTableInstance, cfg.BridgeName, cfg.ContainerIP, cfg.Subnet); err != nil {
		return err
	}

	return c.containerConfigurer.Apply(log, cfg, pid)
}

func (c *configurer) DestroyBridge(log lager.Logger, cfg NetworkConfig) error {
	return c.hostConfigurer.Destroy(cfg)
}

func (c *configurer) DestroyIPTablesRules(log lager.Logger, cfg NetworkConfig) error {
	return c.instanceChainCreator.Destroy(log, cfg.IPTableInstance)
}

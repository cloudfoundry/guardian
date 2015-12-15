package kawasaki

import (
	"net"
	"os"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . NetnsExecer
type NetnsExecer interface {
	Exec(netnsFD *os.File, cb func() error) error
}

type configurer struct {
	hostConfigurer     HostConfigurer
	containerApplier   ContainerApplier
	ipTablesConfigurer IPTablesConfigurer
	nsExecer           NetnsExecer
}

//go:generate counterfeiter . HostConfigurer
type HostConfigurer interface {
	Apply(cfg NetworkConfig, netnsFD *os.File) error
	Destroy(cfg NetworkConfig) error
}

//go:generate counterfeiter . IPTablesConfigurer
type IPTablesConfigurer interface {
	Apply(instanceChain, bridgeName string, ip net.IP, network *net.IPNet) error
	Destroy(instanceChain string) error
}

//go:generate counterfeiter . ContainerApplier
type ContainerApplier interface {
	Apply(cfg NetworkConfig) error
}

func NewConfigurer(hostConfigurer HostConfigurer, containerApplier ContainerApplier, ipTablesConfigurer IPTablesConfigurer, nsExecer NetnsExecer) *configurer {
	return &configurer{
		hostConfigurer:     hostConfigurer,
		containerApplier:   containerApplier,
		ipTablesConfigurer: ipTablesConfigurer,
		nsExecer:           nsExecer,
	}
}

func (c *configurer) Apply(log lager.Logger, cfg NetworkConfig, nsPath string) error {
	fd, err := os.Open(nsPath)
	if err != nil {
		return err
	}
	defer fd.Close()

	if err := c.hostConfigurer.Apply(cfg, fd); err != nil {
		return err
	}

	if err := c.ipTablesConfigurer.Apply(cfg.IPTableChain, cfg.BridgeName, cfg.ContainerIP, cfg.Subnet); err != nil {
		return err
	}

	return c.nsExecer.Exec(fd, func() error {
		return c.containerApplier.Apply(cfg)
	})
}

func (c *configurer) Destroy(log lager.Logger, cfg NetworkConfig) error {
	if err := c.ipTablesConfigurer.Destroy(cfg.IPTableChain); err != nil {
		return err
	}

	if err := c.hostConfigurer.Destroy(cfg); err != nil {
		return err
	}

	return nil
}

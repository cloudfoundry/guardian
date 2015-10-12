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

type applier struct {
	hostConfigApplier      HostApplier
	containerConfigApplier ContainerApplier
	iptablesApplier        IPTablesApplier
	nsExecer               NetnsExecer
}

//go:generate counterfeiter . HostApplier
type HostApplier interface {
	Apply(cfg NetworkConfig, netnsFD *os.File) error
}

//go:generate counterfeiter . IPTablesApplier
type IPTablesApplier interface {
	Apply(instanceChain, bridgeName string, ip net.IP, network *net.IPNet) error
	Teardown(instanceChain string) error
}

//go:generate counterfeiter . ContainerApplier
type ContainerApplier interface {
	Apply(cfg NetworkConfig) error
}

func NewConfigApplier(hostConfigApplier HostApplier, containerConfigApplier ContainerApplier, iptablesApplier IPTablesApplier, nsExecer NetnsExecer) *applier {
	return &applier{
		hostConfigApplier:      hostConfigApplier,
		containerConfigApplier: containerConfigApplier,
		iptablesApplier:        iptablesApplier,
		nsExecer:               nsExecer,
	}
}

func (c *applier) Apply(log lager.Logger, cfg NetworkConfig, nsPath string) error {
	fd, err := os.Open(nsPath)
	if err != nil {
		return err
	}

	if err := c.hostConfigApplier.Apply(cfg, fd); err != nil {
		return err
	}

	if err := c.iptablesApplier.Apply(cfg.IPTableChain, cfg.BridgeName, cfg.ContainerIP, cfg.Subnet); err != nil {
		return err
	}

	return c.nsExecer.Exec(fd, func() error {
		return c.containerConfigApplier.Apply(cfg)
	})
}

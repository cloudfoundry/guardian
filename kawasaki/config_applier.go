package kawasaki

import (
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
	nsExecer               NetnsExecer
}

//go:generate counterfeiter . HostApplier
type HostApplier interface {
	Apply(cfg NetworkConfig, netnsFD *os.File) error
}

//go:generate counterfeiter . ContainerApplier
type ContainerApplier interface {
	Apply(cfg NetworkConfig) error
}

func NewConfigApplier(hostConfigApplier HostApplier, containerConfigApplier ContainerApplier, nsExecer NetnsExecer) *applier {
	return &applier{
		hostConfigApplier:      hostConfigApplier,
		containerConfigApplier: containerConfigApplier,
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

	return c.nsExecer.Exec(fd, func() error {
		return c.containerConfigApplier.Apply(cfg)
	})
}

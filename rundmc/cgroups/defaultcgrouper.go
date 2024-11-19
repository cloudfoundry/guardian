package cgroups

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
)

type DefaultCgrouper struct {
	cgroupRoot string
}

func NewDefaultCgrouper(cgroupRoot string) DefaultCgrouper {
	return DefaultCgrouper{
		cgroupRoot: cgroupRoot,
	}
}

func (c DefaultCgrouper) PrepareCgroups(handle string) error {
	cgroupPath := filepath.Join(c.cgroupRoot, handle)
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return err
	}
	return nil
}

func (DefaultCgrouper) CleanupCgroups(string) error {
	return nil
}

func (DefaultCgrouper) ReadBadCgroupUsage(string) (garden.ContainerCPUStat, error) {
	return garden.ContainerCPUStat{}, nil
}

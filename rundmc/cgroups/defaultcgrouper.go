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
	return nil
}

func (c DefaultCgrouper) CleanupCgroups(handle string) error {
	if err := os.RemoveAll(filepath.Join(c.cgroupRoot, handle)); err != nil {
		return err
	}
	return nil
}

func (DefaultCgrouper) ReadBadCgroupUsage(string) (garden.ContainerCPUStat, error) {
	return garden.ContainerCPUStat{}, nil
}

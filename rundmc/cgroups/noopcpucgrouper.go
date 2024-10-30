package cgroups

import "code.cloudfoundry.org/garden"

type NoopCPUCgrouper struct{}

func (NoopCPUCgrouper) PrepareCgroups(string) error {
	return nil
}

func (NoopCPUCgrouper) CleanupCgroups(string) error {
	return nil
}

func (NoopCPUCgrouper) ReadBadCgroupUsage(string) (garden.ContainerCPUStat, error) {
	return garden.ContainerCPUStat{}, nil
}

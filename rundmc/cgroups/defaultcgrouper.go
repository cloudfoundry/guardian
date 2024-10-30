package cgroups

import (
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
	// cgroupPath := filepath.Join(c.cgroupRoot, handle)
	// if err := os.MkdirAll(cgroupPath, 0755); err != nil {
	// 	return err
	// }
	// if err := os.WriteFile(filepath.Join(cgroupPath, "cgroup.subtree_control"), []byte("+cpuset +cpu +io +memory +hugetlb +pids +rdma +misc"), 0755); err != nil {
	// 	return err
	// }
	return nil
}

func (DefaultCgrouper) CleanupCgroups(string) error {
	return nil
}

func (DefaultCgrouper) ReadBadCgroupUsage(string) (garden.ContainerCPUStat, error) {
	return garden.ContainerCPUStat{}, nil
}

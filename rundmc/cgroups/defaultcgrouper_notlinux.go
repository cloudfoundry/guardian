//go:build !linux

package cgroups

func (DefaultCgrouper) PropagateContainerMemoryLimit(_ string, _ int64, _ bool) error {
	return nil
}

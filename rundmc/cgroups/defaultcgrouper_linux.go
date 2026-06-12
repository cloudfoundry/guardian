// @AI-Generated
// Generated in whole or in part by Cursor with a mix of different LLM models (Auto select mode)
// Description:
// 2026-06-11: Propagate container memory limit to sandbox cgroup for PEA inheritance on cgroupsv2
// 2026-06-11: Also propagate memory.swap.max=0 so PEAs cannot bypass the limit via swap

package cgroups

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PropagateContainerMemoryLimit writes the container memory limit to the sandbox
// cgroup so that PEAs (which are siblings of the container's init sub-cgroup)
// inherit the same memory constraint via the parent cgroup hierarchy.
// This is only needed on cgroupsv2 where the container init process lives at
// sandbox/init, leaving sandbox itself with no explicit memory.max.
// When disableSwapLimit is false, memory.swap.max is also set to 0 so that
// PEAs cannot bypass the memory limit by swapping.
func (c DefaultCgrouper) PropagateContainerMemoryLimit(handle string, memoryLimit int64, disableSwapLimit bool) error {
	if !IsCgroup2UnifiedMode() || memoryLimit <= 0 {
		return nil
	}
	sandboxCgroupPath := filepath.Join(c.cgroupRoot, handle)
	if err := os.WriteFile(
		filepath.Join(sandboxCgroupPath, "memory.max"),
		[]byte(fmt.Sprintf("%d", memoryLimit)),
		0644,
	); err != nil {
		return err
	}
	if !disableSwapLimit {
		err := os.WriteFile(filepath.Join(sandboxCgroupPath, "memory.swap.max"), []byte("0"), 0644)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// @AI-Generated
// Generated in whole or in part by Cursor with a mix of different LLM models (Auto select mode)
// Description:
// 2026-06-11: Propagate container memory limit to sandbox cgroup for PEA inheritance on cgroupsv2
// 2026-06-11: Also propagate memory.swap.max=0 so PEAs cannot bypass the limit via swap
// 2026-06-12: Remove redundant memory.max write - runc already writes it; only memory.swap.max=0 is needed

package cgroups

import (
	"errors"
	"os"
	"path/filepath"
)

// PropagateContainerMemoryLimit sets memory.swap.max=0 on the container sandbox
// cgroup so that PEAs (which run as descendants of the container cgroup) cannot
// bypass the container memory limit by using swap.
// runc already writes memory.max when the container spec includes a memory limit,
// so we only need to set memory.swap.max=0 here when swap limiting is active.
func (c DefaultCgrouper) PropagateContainerMemoryLimit(handle string, memoryLimit int64, disableSwapLimit bool) error {
	if !IsCgroup2UnifiedMode() || memoryLimit <= 0 || disableSwapLimit {
		return nil
	}
	sandboxCgroupPath := filepath.Join(c.cgroupRoot, handle)
	err := os.WriteFile(filepath.Join(sandboxCgroupPath, "memory.swap.max"), []byte("0"), 0644)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

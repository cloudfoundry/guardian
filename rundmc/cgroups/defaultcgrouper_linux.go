// @AI-Generated
// Generated in whole or in part by Cursor with a mix of different LLM models (Auto select mode)
// Description:
// 2026-06-11: Propagate container memory limit to sandbox cgroup for PEA inheritance on cgroupsv2
// 2026-06-11: Also propagate memory.swap.max=0 so PEAs cannot bypass the limit via swap
// 2026-06-12: Remove redundant memory.max write - runc already writes it; only memory.swap.max=0 is needed
// 2026-06-15: Move container init process to 'init' sub-cgroup so sandbox can enable subtree controllers for PEAs

package cgroups

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// PropagateContainerMemoryLimit prepares the sandbox cgroup so that PEA
// sub-cgroups can have proper memory accounting in cgroupsv2.
//
// In cgroupsv2, runc puts the container init process directly in the sandbox
// cgroup and sets memory.max there. When a PEA is later created as a child
// cgroup, the sandbox is in "domain invalid" state (has both direct processes
// and children), causing runc to place the PEA cgroup in "threaded" mode.
// Threaded mode disables domain controllers (memory), so runc stats returns
// zero for all memory fields.
//
// Fix: move the container's init process(es) to a 'init' sub-cgroup so the
// sandbox cgroup has no direct processes, then enable domain controllers in
// sandbox/cgroup.subtree_control. Subsequent PEA sub-cgroups will be created
// in domain mode and will have working memory accounting.
func (c DefaultCgrouper) PropagateContainerMemoryLimit(handle string, memoryLimit int64, disableSwapLimit bool) error {
	if !IsCgroup2UnifiedMode() || memoryLimit <= 0 || disableSwapLimit {
		return nil
	}
	sandboxCgroupPath := filepath.Join(c.cgroupRoot, handle)

	if err := prepareSandboxSubtreeForPeas(sandboxCgroupPath); err != nil {
		// If the sandbox cgroup doesn't exist (e.g. fake runtime in GQT tests),
		// skip silently. Any other error is real.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	err := os.WriteFile(filepath.Join(sandboxCgroupPath, "memory.swap.max"), []byte("0"), 0644)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// prepareSandboxSubtreeForPeas moves any processes directly in the sandbox
// cgroup into an 'init' sub-cgroup, then enables all available controllers in
// sandbox/cgroup.subtree_control. This removes the "no internal process"
// constraint so that child PEA cgroups can be created as domain cgroups with
// working memory accounting.
func prepareSandboxSubtreeForPeas(sandboxCgroupPath string) error {
	procsData, err := os.ReadFile(filepath.Join(sandboxCgroupPath, "cgroup.procs"))
	if err != nil {
		return err
	}

	pids := strings.Fields(string(procsData))
	if len(pids) > 0 {
		initCgroupPath := filepath.Join(sandboxCgroupPath, "init")
		if err := os.MkdirAll(initCgroupPath, 0755); err != nil {
			return err
		}
		for _, pid := range pids {
			if err := os.WriteFile(filepath.Join(initCgroupPath, "cgroup.procs"), []byte(pid), 0644); err != nil {
				return err
			}
		}
	}

	// Enable all available controllers in the sandbox subtree so that child
	// cgroups (PEAs) can use domain controllers like memory.
	controllersData, err := os.ReadFile(filepath.Join(sandboxCgroupPath, "cgroup.controllers"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, ctr := range strings.Fields(string(controllersData)) {
		_ = os.WriteFile(filepath.Join(sandboxCgroupPath, "cgroup.subtree_control"), []byte("+"+ctr), 0644)
	}

	return nil
}

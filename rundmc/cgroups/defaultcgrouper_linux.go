// @AI-Generated
// Generated in whole or in part by Cursor with a mix of different LLM models (Auto select mode)
// Description:
// 2026-06-11: Propagate container memory limit to sandbox cgroup for PEA inheritance on cgroupsv2
// 2026-06-11: Also propagate memory.swap.max=0 so PEAs cannot bypass the limit via swap
// 2026-06-12: Remove redundant memory.max write - runc already writes it; only memory.swap.max=0 is needed
// 2026-06-15: Move container init process to 'init' sub-cgroup so sandbox can enable subtree controllers for PEAs

package cgroups

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
// #region agent log
func debugLog(msg string, data map[string]interface{}) {
	entry := map[string]interface{}{
		"sessionId": "7b1df1",
		"timestamp": time.Now().UnixMilli(),
		"location":  "defaultcgrouper_linux.go",
		"message":   msg,
		"data":      data,
	}
	b, _ := json.Marshal(entry)
	f, err := os.OpenFile("/Users/ad008239/workspace/.cursor/debug-7b1df1.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", b)
}

// #endregion

func (c DefaultCgrouper) PropagateContainerMemoryLimit(handle string, memoryLimit int64, disableSwapLimit bool) error {
	// #region agent log
	debugLog("PropagateContainerMemoryLimit called", map[string]interface{}{
		"handle": handle, "memoryLimit": memoryLimit, "disableSwapLimit": disableSwapLimit,
		"isCgroup2": IsCgroup2UnifiedMode(), "hypothesisId": "A",
	})
	// #endregion

	if !IsCgroup2UnifiedMode() || memoryLimit <= 0 || disableSwapLimit {
		debugLog("PropagateContainerMemoryLimit early return", map[string]interface{}{
			"reason":       fmt.Sprintf("cgroup2=%v memLimit=%d disableSwap=%v", IsCgroup2UnifiedMode(), memoryLimit, disableSwapLimit),
			"hypothesisId": "A",
		})
		return nil
	}
	sandboxCgroupPath := filepath.Join(c.cgroupRoot, handle)

	if err := prepareSandboxSubtreeForPeas(sandboxCgroupPath); err != nil {
		// If the sandbox cgroup doesn't exist (e.g. fake runtime in GQT tests),
		// skip silently. Any other error is real.
		debugLog("prepareSandboxSubtreeForPeas error", map[string]interface{}{
			"err": err.Error(), "path": sandboxCgroupPath, "hypothesisId": "B",
		})
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	err := os.WriteFile(filepath.Join(sandboxCgroupPath, "memory.swap.max"), []byte("0"), 0644)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		debugLog("memory.swap.max write error", map[string]interface{}{
			"err": err.Error(), "hypothesisId": "C",
		})
		return err
	}

	// #region agent log
	debugLog("PropagateContainerMemoryLimit done", map[string]interface{}{
		"handle": handle, "hypothesisId": "A",
	})
	// #endregion

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
	// #region agent log
	debugLog("prepareSandboxSubtreeForPeas", map[string]interface{}{
		"sandboxPath": sandboxCgroupPath, "pids": pids, "hypothesisId": "B",
	})
	// #endregion

	if len(pids) > 0 {
		initCgroupPath := filepath.Join(sandboxCgroupPath, "init")
		if mkdirErr := os.MkdirAll(initCgroupPath, 0755); mkdirErr != nil {
			debugLog("mkdir init cgroup error", map[string]interface{}{"err": mkdirErr.Error(), "hypothesisId": "B"})
			return mkdirErr
		}
		for _, pid := range pids {
			if writeErr := os.WriteFile(filepath.Join(initCgroupPath, "cgroup.procs"), []byte(pid), 0644); writeErr != nil {
				debugLog("move pid error", map[string]interface{}{"pid": pid, "err": writeErr.Error(), "hypothesisId": "B"})
				return writeErr
			}
		}
	}

	// Enable all available controllers in the sandbox subtree so that child
	// cgroups (PEAs) can use domain controllers like memory.
	controllersData, err := os.ReadFile(filepath.Join(sandboxCgroupPath, "cgroup.controllers"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	controllers := strings.Fields(string(controllersData))
	// #region agent log
	debugLog("enabling subtree controllers", map[string]interface{}{
		"sandboxPath": sandboxCgroupPath, "controllers": controllers, "hypothesisId": "B",
	})
	// #endregion
	var subtreeErrs []string
	for _, ctr := range controllers {
		if writeErr := os.WriteFile(filepath.Join(sandboxCgroupPath, "cgroup.subtree_control"), []byte("+"+ctr), 0644); writeErr != nil {
			subtreeErrs = append(subtreeErrs, fmt.Sprintf("%s:%v", ctr, writeErr))
		}
	}
	// #region agent log
	subtreeCtrlData, _ := os.ReadFile(filepath.Join(sandboxCgroupPath, "cgroup.subtree_control"))
	cgroupType, _ := os.ReadFile(filepath.Join(sandboxCgroupPath, "cgroup.type"))
	debugLog("after enabling subtree controllers", map[string]interface{}{
		"subtreeControl": strings.TrimSpace(string(subtreeCtrlData)),
		"cgroupType":     strings.TrimSpace(string(cgroupType)),
		"writeErrors":    subtreeErrs,
		"hypothesisId":   "B",
	})
	// #endregion

	return nil
}

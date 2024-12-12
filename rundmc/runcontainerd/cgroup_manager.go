package runcontainerd

import (
	"encoding/json"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . CgroupManager
type CgroupManager interface {
	SetUseMemoryHierarchy(handle string) error
	SetUnifiedResources(bundle goci.Bndl) error
}

type cgroupManager struct {
	runcRoot  string
	namespace string
}

type containerState struct {
	CgroupPaths cgroupPaths `json:"cgroup_paths"`
}

type cgroupPaths struct {
	Memory string
}

func NewCgroupManager(runcRoot, namespace string) CgroupManager {
	return cgroupManager{
		runcRoot:  runcRoot,
		namespace: namespace,
	}
}

func (m cgroupManager) SetUseMemoryHierarchy(handle string) error {
	statePath := filepath.Join(m.runcRoot, m.namespace, handle, "state.json")
	stateFile, err := os.Open(statePath)
	if err != nil {
		return err
	}
	defer stateFile.Close()

	var state containerState
	err = json.NewDecoder(stateFile).Decode(&state)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(state.CgroupPaths.Memory, "memory.use_hierarchy"), []byte("1"), os.ModePerm)
}

func (m cgroupManager) SetUnifiedResources(bundle goci.Bndl) error {
	// we are using UnifiedMountpoint because fs2.CreateCgroupPath checks that path starts with it
	cgroupPath := filepath.Join(fs2.UnifiedMountpoint, bundle.Spec.Linux.CgroupsPath)

	resources := convertSpecResourcesToCgroupResources(bundle.Spec.Linux.Resources)
	if resources != nil {
		cgroupManager, err := fs2.NewManager(&configs.Cgroup{}, cgroupPath)
		if err != nil {
			return err
		}
		err = fs2.CreateCgroupPath(cgroupPath, &configs.Cgroup{})
		if err != nil {
			return err
		}
		err = cgroupManager.Set(resources)
		if err != nil {
			return err
		}
	}
	return nil
}

func convertSpecResourcesToCgroupResources(specResources *specs.LinuxResources) *configs.Resources {
	if specResources == nil {
		return nil
	}

	resources := &configs.Resources{}
	resources.Unified = specResources.Unified

	if specResources.CPU != nil {
		if specResources.CPU.Shares != nil {
			resources.CpuShares = *specResources.CPU.Shares
		}
		if specResources.CPU.Quota != nil {
			resources.CpuQuota = *specResources.CPU.Quota
		}
		if specResources.CPU.Period != nil {
			resources.CpuPeriod = *specResources.CPU.Period
		}
	}
	if specResources.Memory != nil {
		if specResources.Memory.Limit != nil {
			resources.Memory = *specResources.Memory.Limit
		}
		if specResources.Memory.Swap != nil {
			resources.MemorySwap = *specResources.Memory.Swap
		}
	}

	return resources
}

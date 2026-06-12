// @AI-Generated
// Generated in whole or in part by Cursor with a mix of different LLM models (Auto select mode)
// Description:
// 2026-06-11: Use sandbox-id/init sub-cgroup for container init process on cgroupsv2 to allow PEA memory tracking

package bundlerules

import (
	"path/filepath"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

// ContainerInitSubCgroup is the sub-cgroup name used on cgroupsv2 to house the
// container's init process. Using a sub-cgroup keeps the parent (sandbox-id)
// free of direct processes, which allows domain controllers such as memory to
// remain in its cgroup.subtree_control. PEA sub-cgroups created alongside the
// init sub-cgroup then inherit memory accounting.
const ContainerInitSubCgroup = "init"

type CGroupPath struct {
	Path string
	// UseInitSubCgroup should be set to true on cgroupsv2 hosts. When true the
	// container init process is placed in a "init" sub-cgroup of the sandbox
	// cgroup so that the parent cgroup (sandbox-id) remains free of direct
	// processes. This is required for domain controllers (e.g. memory) to be
	// kept in the parent's cgroup.subtree_control, which in turn enables memory
	// accounting for PEA sub-cgroups.
	UseInitSubCgroup bool
}

func (r CGroupPath) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	if spec.Privileged {
		return bndl, nil
	}

	var basePath string
	if spec.CgroupPath != "" {
		basePath = filepath.Join(r.Path, spec.CgroupPath)
	} else {
		basePath = filepath.Join(r.Path, spec.Handle)
	}

	if r.UseInitSubCgroup {
		return bndl.WithCGroupPath(filepath.Join(basePath, ContainerInitSubCgroup)), nil
	}

	return bndl.WithCGroupPath(basePath), nil
}

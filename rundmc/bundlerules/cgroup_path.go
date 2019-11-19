package bundlerules

import (
	"path/filepath"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type CGroupPath struct {
	Path string
}

func (r CGroupPath) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	if spec.Privileged {
		return bndl, nil
	}

	if spec.CgroupPath != "" {
		return bndl.WithCGroupPath(filepath.Join(r.Path, cgroups.GoodCgroupName, spec.CgroupPath)), nil
	}

	return bndl.WithCGroupPath(filepath.Join(r.Path, cgroups.GoodCgroupName, spec.Handle)), nil
}

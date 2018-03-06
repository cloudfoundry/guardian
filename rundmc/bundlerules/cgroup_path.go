package bundlerules

import (
	"path/filepath"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type CGroupPath struct {
	Path string
}

func (r CGroupPath) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	if spec.Privileged {
		return bndl, nil
	}

	if spec.CgroupPath != "" {
		return bndl.WithCGroupPath(filepath.Join(r.Path, spec.CgroupPath)), nil
	}

	return bndl.WithCGroupPath(filepath.Join(r.Path, spec.Handle)), nil
}

package bundlerules

import (
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type CGroupPath struct {
	Path string
}

func (r CGroupPath) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	if spec.Privileged {
		return bndl, nil
	}

	return bndl.WithCGroupPath(filepath.Join(r.Path, spec.Handle)), nil
}

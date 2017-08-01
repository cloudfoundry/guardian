package bundlerules

import (
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type CGroupPath struct {
	GardenCgroup string
}

func (r CGroupPath) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	return bndl.WithCGroupPath(filepath.Join(r.GardenCgroup, spec.Hostname)), nil
}

package bundlerules

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type RootFSPropagation struct {
}

func (l RootFSPropagation) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	rootfsPropagation := spec.RootFSPropagation
	return bndl.WithRootFSPropagation(rootfsPropagation), nil
}

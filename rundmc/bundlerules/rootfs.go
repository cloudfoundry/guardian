package bundlerules

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type RootFS struct {
}

func (r RootFS) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	return bndl.WithRootFS(spec.BaseConfig.Root.Path), nil
}

package bundlerules

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

type Env struct {
}

func (r Env) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl {
	process := bndl.Process()
	process.Env = spec.Env
	return bndl.WithProcess(process)
}

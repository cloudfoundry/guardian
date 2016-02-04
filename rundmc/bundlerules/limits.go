package bundlerules

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
)

type Limits struct {
}

func (l Limits) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	return bndl.WithResources(&specs.Resources{
		Memory: specs.Memory{Limit: int64(spec.Limits.Memory.LimitInBytes)},
	})
}

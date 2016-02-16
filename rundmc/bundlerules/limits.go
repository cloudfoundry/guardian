package bundlerules

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/opencontainers/specs"
)

type Limits struct {
}

func (l Limits) Apply(bndl *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
	limit := uint64(spec.Limits.Memory.LimitInBytes)
	return bndl.WithMemoryLimit(specs.Memory{Limit: &limit})
}

package bundlerules

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Limits struct {
}

func (l Limits) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl {
	limit := uint64(spec.Limits.Memory.LimitInBytes)
	bndl = bndl.WithMemoryLimit(specs.Memory{Limit: &limit, Swap: &limit})
	shares := uint64(spec.Limits.CPU.LimitInShares)
	return bndl.WithCPUShares(specs.CPU{Shares: &shares})
}

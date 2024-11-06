package bundlerules

import (
	"math"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Windows struct{}

func (w Windows) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	if spec.BaseConfig.Windows == nil {
		return bndl, nil
	}

	bndl = bndl.WithWindows(*spec.BaseConfig.Windows)

	limit := uint64(spec.Limits.Memory.LimitInBytes)
	bndl = bndl.WithWindowsMemoryLimit(specs.WindowsMemoryResources{Limit: &limit})

	//lint:ignore SA1019 - we still specify this to make the deprecated logic work until we get rid of the code in garden
	shares := spec.Limits.CPU.LimitInShares
	if spec.Limits.CPU.Weight > 0 {
		shares = spec.Limits.CPU.Weight
	}

	var sharesUint uint16
	if shares > math.MaxUint16 {
		sharesUint = math.MaxUint16
	} else {
		sharesUint = uint16(shares)
	}

	bndl = bndl.WithWindowsCPUShares(specs.WindowsCPUResources{Shares: &sharesUint})

	return bndl, nil
}

package bundlerules

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Windows struct{}

func (w Windows) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	limit := uint64(spec.Limits.Memory.LimitInBytes)
	bndl = bndl.WithWindowsMemoryLimit(specs.WindowsMemoryResources{Limit: &limit})
	if spec.BaseConfig.Spec.Windows != nil {
		bndl = bndl.WithWindowsLayerFolders(spec.BaseConfig.Spec.Windows.LayerFolders)
	}

	return bndl, nil
}

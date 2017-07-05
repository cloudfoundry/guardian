package bundlerules

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Mounts struct {
}

func (b Mounts) Apply(bndl goci.Bndl, spec gardener.DesiredContainerSpec, containerDir string) (goci.Bndl, error) {
	var mounts []specs.Mount
	for _, m := range spec.BindMounts {
		modeOpt := "ro"
		if m.Mode == garden.BindMountModeRW {
			modeOpt = "rw"
		}

		mounts = append(mounts, specs.Mount{
			Destination: m.DstPath,
			Source:      m.SrcPath,
			Type:        "bind",
			Options:     []string{"bind", modeOpt},
		})
	}

	bndl = bndl.WithPrependedMounts(spec.DesiredImageSpecMounts...)
	bndl = bndl.WithMounts(mounts...)

	return bndl, nil
}

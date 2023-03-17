package bundlerules

import (
	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/v3"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Mounts struct {
	logger             lager.Logger
	mountOptionsGetter rundmc.MountOptionsGetter
}

func NewMounts(logger lager.Logger, mountOptionsGetter rundmc.MountOptionsGetter) Mounts {
	return Mounts{
		logger:             logger,
		mountOptionsGetter: mountOptionsGetter,
	}
}

func (b Mounts) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec) (goci.Bndl, error) {
	var mounts []specs.Mount
	for _, m := range spec.BindMounts {
		mounts = append(mounts, specs.Mount{
			Destination: m.DstPath,
			Source:      m.SrcPath,
			Type:        "bind",
			Options:     b.buildMountOptions(m),
		})
	}

	return bndl.WithPrependedMounts(spec.BaseConfig.Mounts...).WithMounts(mounts...), nil
}

func (b Mounts) buildMountOptions(m garden.BindMount) []string {
	mountOptions := []string{"bind", getMountMode(m)}

	srcMountOptions, err := b.mountOptionsGetter(m.SrcPath)
	if err != nil {
		b.logger.Info("failed to get mount options, assuming no additional mount options", lager.Data{"error": err})
		srcMountOptions = []string{}
	}

	return append(mountOptions, filterModeOption(srcMountOptions)...)
}

func getMountMode(m garden.BindMount) string {
	if m.Mode == garden.BindMountModeRW {
		return "rw"
	}

	return "ro"
}

func filterModeOption(mountOptions []string) []string {
	filteredOptions := []string{}
	for _, o := range mountOptions {
		if o == "rw" || o == "ro" || o == "bind" {
			continue
		}
		filteredOptions = append(filteredOptions, o)
	}

	return filteredOptions
}

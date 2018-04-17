package bundlerules

import (
	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"github.com/docker/docker/pkg/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type Mounts struct {
	MountOptionsGetter rundmc.MountOptionsGetter
	MountInfosProvider func() ([]*mount.Info, error)
}

func (b Mounts) Apply(bndl goci.Bndl, spec spec.DesiredContainerSpec, _ string) (goci.Bndl, error) {
	mountInfos, err := b.MountInfosProvider()
	if err != nil {
		return goci.Bndl{}, err
	}

	var mounts []specs.Mount
	for _, m := range spec.BindMounts {
		mountOptions, err := b.buildMountOptions(m, mountInfos)
		if err != nil {
			return goci.Bndl{}, err
		}

		mounts = append(mounts, specs.Mount{
			Destination: m.DstPath,
			Source:      m.SrcPath,
			Type:        "bind",
			Options:     mountOptions,
		})
	}

	return bndl.WithPrependedMounts(spec.BaseConfig.Mounts...).WithMounts(mounts...), nil
}

func (b Mounts) buildMountOptions(m garden.BindMount, mountInfos []*mount.Info) ([]string, error) {
	mountOptions := []string{"bind", getMountMode(m)}

	srcMountOptions, err := b.MountOptionsGetter(m.SrcPath, mountInfos)
	if err != nil {
		return nil, err
	}
	return append(mountOptions, filterModeOption(srcMountOptions)...), nil
}

func getMountMode(m garden.BindMount) string {
	if m.Mode == garden.BindMountModeRW {
		return "rw"
	}

	return "ro"
}

func filterModeOption(mountOptions []string) []string {
	filteredOptions := []string{}
	for i, o := range mountOptions {
		if o == "rw" || o == "ro" || o == "bind" {
			filteredOptions = append(mountOptions[0:i], mountOptions[i+1:]...)
		}
	}

	return filteredOptions
}

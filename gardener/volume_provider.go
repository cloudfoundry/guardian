package gardener

import (
	"errors"
	"net/url"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const RawRootFSScheme = "raw"

type VolumeProvider struct {
	VolumeCreator VolumeCreator
	VolumeDestroyMetricsGC
}

func NewVolumeProvider(creator VolumeCreator, manager VolumeDestroyMetricsGC) *VolumeProvider {
	return &VolumeProvider{VolumeCreator: creator, VolumeDestroyMetricsGC: manager}
}

type VolumeCreator interface {
	Create(log lager.Logger, handle string, spec rootfs_spec.Spec) (specs.Spec, error)
}

func (v *VolumeProvider) Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error) {
	path := spec.Image.URI
	if path == "" {
		path = spec.RootFSPath
	} else if spec.RootFSPath != "" {
		return specs.Spec{}, errors.New("Cannot provide both Image.URI and RootFSPath")
	}

	rootFSURL, err := url.Parse(path)
	if err != nil {
		return specs.Spec{}, err
	}

	var baseConfig specs.Spec
	if rootFSURL.Scheme == RawRootFSScheme {
		baseConfig.Root = &specs.Root{Path: rootFSURL.Path}
		baseConfig.Process = &specs.Process{}
	} else {
		var err error
		baseConfig, err = v.VolumeCreator.Create(log.Session("volume-creator"), spec.Handle, rootfs_spec.Spec{
			RootFS:     rootFSURL,
			Username:   spec.Image.Username,
			Password:   spec.Image.Password,
			QuotaSize:  int64(spec.Limits.Disk.ByteHard),
			QuotaScope: spec.Limits.Disk.Scope,
			Namespaced: !spec.Privileged,
		})
		if err != nil {
			return specs.Spec{}, err
		}
	}

	return baseConfig, nil
}

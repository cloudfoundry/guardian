package nerdimage

import (
	"context"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type ContainerdVolumizer struct {
	client  *containerd.Client
	context context.Context
}

func NewContainerdVolumizer(client *containerd.Client, context context.Context) *ContainerdVolumizer {
	return &ContainerdVolumizer{client: client, context: context}
}

func (v ContainerdVolumizer) Create(log lager.Logger, spec garden.ContainerSpec) (specs.Spec, error) {
	image, err := v.client.Pull(v.context, spec.Image.URI)
	if err != nil {
		return specs.Spec{}, err
	}

	if err := image.Unpack(v.context, spec.Handle); err != nil {
		return specs.Spec{}, err
	}

	mnts, err := v.client.SnapshotService(spec.Handle).Prepare(v.context, "1", "")
	if err != nil {
		return specs.Spec{}, err
	}

	return specs.Spec{Root: &specs.Root{Path: mnts[0].Source}}, nil
}

func (v ContainerdVolumizer) Destroy(log lager.Logger, handle string) error {
	return nil
}

func (v ContainerdVolumizer) Metrics(log lager.Logger, handle string, namespaced bool) (garden.ContainerDiskStat, error) {
	return garden.ContainerDiskStat{}, nil
}

func (v ContainerdVolumizer) GC(log lager.Logger) error {
	return nil
}

func (v ContainerdVolumizer) Capacity(log lager.Logger) (uint64, error) {
	return 0, nil
}

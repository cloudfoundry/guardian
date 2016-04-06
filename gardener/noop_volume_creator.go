package gardener

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/pivotal-golang/lager"
)

type NoopVolumeCreator struct{}

var ErrGraphDisabled = errors.New("volume graph is disabled")

func (NoopVolumeCreator) Create(lager.Logger, string, rootfs_provider.Spec) (string, []string, error) {
	return "", nil, ErrGraphDisabled
}

func (NoopVolumeCreator) Destroy(lager.Logger, string) error {
	return nil
}

func (NoopVolumeCreator) Metrics(lager.Logger, string) (garden.ContainerDiskStat, error) {
	return garden.ContainerDiskStat{}, nil
}

func (NoopVolumeCreator) GC(lager.Logger) error {
	return nil
}

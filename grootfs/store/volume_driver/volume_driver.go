package volume_driver

import "code.cloudfoundry.org/lager"

//go:generate counterfeiter . VolumeDriver

type VolumeDriver interface {
	Path(logger lager.Logger, id string) (string, error)
	Create(logger lager.Logger, parentID, id string) (string, error)
	DestroyVolume(logger lager.Logger, id string) error
	Volumes(logger lager.Logger) ([]string, error)
}

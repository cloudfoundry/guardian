package overlayxfs

import (
	"errors"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
)

func (d *Driver) InitFilesystem(logger lager.Logger, filesystemPath, storePath string) error {
	return nil
}

func (d *Driver) ConfigureStore(logger lager.Logger, storePath string, ownerUID, ownerGID int) error {
	return nil
}

func (d *Driver) ValidateFileSystem(logger lager.Logger, path string) error {
	return nil
}

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	return "", errors.New("Not compatible with windows")
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID string, id string) (string, error) {
	return "", errors.New("Not compatible with windows")
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	return errors.New("Not compatible with windows")
}

func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	return nil, errors.New("Not compatible with windows")
}

func (d *Driver) CreateImage(logger lager.Logger, spec image_cloner.ImageDriverSpec) (groot.MountInfo, error) {
	return groot.MountInfo{}, errors.New("Not compatible with windows")
}

func (d *Driver) DestroyImage(logger lager.Logger, imagePath string) error {
	return errors.New("Not compatible with windows")
}

func (d *Driver) FetchStats(logger lager.Logger, imagePath string) (groot.VolumeStats, error) {
	return groot.VolumeStats{}, errors.New("Not compatible with windows")
}

func (d *Driver) MoveVolume(logger lager.Logger, from, to string) error {
	return errors.New("Not compatible with windows")
}

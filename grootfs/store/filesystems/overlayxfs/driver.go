package overlayxfs

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
)

const (
	UpperDir  = "diff"
	WorkDir   = "workdir"
	RootfsDir = "rootfs"
)

func New(storePath string) *Driver {
	return &Driver{
		storePath: storePath,
	}
}

type Driver struct {
	storePath string
}

func (d *Driver) VolumePath(logger lager.Logger, id string) (string, error) {
	volPath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, id)
	_, err := os.Stat(volPath)
	if err == nil {
		return volPath, nil
	}

	return "", fmt.Errorf("volume does not exist `%s`: %s", id, err)
}

func (d *Driver) CreateVolume(logger lager.Logger, parentID string, id string) (string, error) {
	logger = logger.Session("overlayxfs-creating-volume", lager.Data{"parentID": parentID, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	volumePath := filepath.Join(d.storePath, store.VOLUMES_DIR_NAME, id)
	if err := os.Mkdir(volumePath, 0700); err != nil {
		logger.Error("creating-volume-dir-failed", err)
		return "", errors.Wrap(err, "creating volume")
	}
	return volumePath, nil
}

func (d *Driver) DestroyVolume(logger lager.Logger, id string) error {
	panic("not implemented")
}

func (d *Driver) Volumes(logger lager.Logger) ([]string, error) {
	panic("not implemented")
}

func (d *Driver) CreateImage(logger lager.Logger, fromPath string, imagePath string) error {
	logger = logger.Session("overlayxfs-creating-image", lager.Data{"fromPath": fromPath, "imagePath": imagePath})
	logger.Info("start")
	defer logger.Info("end")

	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		logger.Error("image-path-not-found", err)
		return errors.Wrap(err, "image path does not exist")
	}

	if _, err := os.Stat(fromPath); os.IsNotExist(err) {
		logger.Error("from-path-not-found", err)
		return errors.Wrap(err, "source path does not exist")
	}

	upperDir := filepath.Join(imagePath, UpperDir)
	workDir := filepath.Join(imagePath, WorkDir)
	rootfsDir := filepath.Join(imagePath, RootfsDir)

	if err := os.Mkdir(upperDir, 0700); err != nil {
		logger.Error("creating-upperdir-folder-failed", err)
		return errors.Wrap(err, "creating upperdir folder")
	}

	if err := os.Mkdir(workDir, 0700); err != nil {
		logger.Error("creating-workdir-folder-failed", err)
		return errors.Wrap(err, "creating workdir folder")
	}

	if err := os.Mkdir(rootfsDir, 0700); err != nil {
		logger.Error("creating-rootfs-folder-failed", err)
		return errors.Wrap(err, "creating rootfs folder")
	}

	mountData := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", fromPath, upperDir, workDir)
	if err := syscall.Mount("overlay", rootfsDir, "overlay", 0, mountData); err != nil {
		logger.Error("mounting-overlay-to-rootfs-failed", err, lager.Data{"mountData": mountData, "rootfsDir": rootfsDir})
		return errors.Wrap(err, "mounting overlay")
	}

	return nil
}

func (d *Driver) DestroyImage(logger lager.Logger, imagePath string) error {
	logger = logger.Session("overlayxfs-destroying-image", lager.Data{"imagePath": imagePath})
	logger.Info("start")
	defer logger.Info("end")

	if err := syscall.Unmount(filepath.Join(imagePath, RootfsDir), 0); err != nil {
		logger.Error("unmounting-rootfs-folder-failed", err)
		return errors.Wrap(err, "unmounting rootfs folder")
	}
	if err := os.Remove(filepath.Join(imagePath, RootfsDir)); err != nil {
		logger.Error("removing-rootfs-folder-failed", err)
		return errors.Wrap(err, "deleting rootfs folder")
	}
	if err := os.RemoveAll(filepath.Join(imagePath, WorkDir)); err != nil {
		logger.Error("removing-workdir-folder-failed", err)
		return errors.Wrap(err, "deleting workdir folder")
	}
	if err := os.RemoveAll(filepath.Join(imagePath, UpperDir)); err != nil {
		logger.Error("removing-upperdir-folder-failed", err)
		return errors.Wrap(err, "deleting upperdir folder")
	}

	return nil
}

func (d *Driver) ApplyDiskLimit(logger lager.Logger, path string, diskLimit int64, exclusive bool) error {
	panic("not implemented")
}

func (d *Driver) FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error) {
	panic("not implemented")
}

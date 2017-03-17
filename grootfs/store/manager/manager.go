package manager

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	"github.com/pkg/errors"
)

type Manager struct {
	storePath    string
	imageDriver  image_cloner.ImageDriver
	volumeDriver base_image_puller.VolumeDriver
	storeDriver  store.StoreDriver
	locksmith    groot.Locksmith
}

func New(storePath string, locksmith groot.Locksmith, volumeDriver base_image_puller.VolumeDriver, imageDriver image_cloner.ImageDriver, storeDriver store.StoreDriver) *Manager {
	return &Manager{
		storePath:    storePath,
		volumeDriver: volumeDriver,
		imageDriver:  imageDriver,
		storeDriver:  storeDriver,
		locksmith:    locksmith,
	}
}

func (m *Manager) InitStore(logger lager.Logger) error {
	logger = logger.Session("store-manager-init-store")
	logger.Debug("starting")
	defer logger.Debug("ending")

	stat, err := os.Stat(m.storePath)
	if err == nil && stat.IsDir() {
		logger.Error("store-path-already-exists", errors.Errorf("%s already exists", m.storePath))
		return errors.Errorf("store already initialized at path %s", m.storePath)
	}

	if err := m.storeDriver.ValidateFileSystem(logger, filepath.Dir(m.storePath)); err != nil {
		logger.Error("store-path-validation-failed", err)
		return errors.Wrap(err, "validating store path filesystem")
	}

	if err := os.MkdirAll(m.storePath, 0755); err != nil {
		logger.Error("init-store-failed", err, lager.Data{"storePath": m.storePath})
		return errors.Wrap(err, "initializing store")
	}
	return nil
}

func (m *Manager) DeleteStore(logger lager.Logger) error {
	logger = logger.Session("store-manager-delete-store")
	logger.Debug("starting")
	defer logger.Debug("ending")

	fileLock, err := m.locksmith.Lock(groot.GlobalLockKey)
	if err != nil {
		logger.Error("locking-failed", err)
		return errors.Wrap(err, "requesting lock")
	}
	defer m.locksmith.Unlock(fileLock)

	existingImages, err := m.images()
	if err != nil {
		return err
	}

	for _, image := range existingImages {
		if err := m.imageDriver.DestroyImage(logger, image); err != nil {
			logger.Error("destroing-image-failed", err, lager.Data{"image": image})
			return errors.Wrapf(err, "destroying image %s", image)
		}
	}

	existingVolumes, err := m.volumes()
	if err != nil {
		return err
	}

	for _, volume := range existingVolumes {
		if err := m.volumeDriver.DestroyVolume(logger, volume); err != nil {
			logger.Error("destroing-volume-failed", err, lager.Data{"volume": volume})
			return errors.Wrapf(err, "destroying volume %s", volume)
		}
	}

	if err := os.RemoveAll(m.storePath); err != nil {
		logger.Error("deleting-store-path-failed", err, lager.Data{"storePath": m.storePath})
		return errors.Wrapf(err, "deleting store path")
	}

	return nil
}

func (m *Manager) images() ([]string, error) {
	imagesPath := filepath.Join(m.storePath, store.ImageDirName)
	images, err := ioutil.ReadDir(imagesPath)
	if err != nil {
		return nil, errors.Wrap(err, "listing images")
	}

	imagePaths := []string{}
	for _, file := range images {
		imagePaths = append(imagePaths, filepath.Join(imagesPath, file.Name()))
	}

	return imagePaths, nil
}

func (m *Manager) volumes() ([]string, error) {
	volumesPath := filepath.Join(m.storePath, store.VolumesDirName)
	volumes, err := ioutil.ReadDir(volumesPath)
	if err != nil {
		return nil, errors.Wrap(err, "listing volumes")
	}

	volumeIds := []string{}
	for _, file := range volumes {
		volumeIds = append(volumeIds, file.Name())
	}

	return volumeIds, nil
}

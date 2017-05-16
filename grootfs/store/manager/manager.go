package manager

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/store/image_cloner"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const MinStoreSizeBytes = 1024 * 1024 * 200

//go:generate counterfeiter . StoreDriver
type StoreDriver interface {
	ConfigureStore(logger lager.Logger, storePath string, ownerUID, ownerGID int) error
	ValidateFileSystem(logger lager.Logger, path string) error
	InitFilesystem(logger lager.Logger, filesystemPath, storePath string) error
}

type Manager struct {
	storePath    string
	imageDriver  image_cloner.ImageDriver
	volumeDriver base_image_puller.VolumeDriver
	storeDriver  StoreDriver
	locksmith    groot.Locksmith
}

//go:generate counterfeiter . NamespaceWriter
type NamespaceWriter interface {
	Write(storePath string, uidMappings, gidMappings []groot.IDMappingSpec) error
}

type InitSpec struct {
	UIDMappings     []groot.IDMappingSpec
	GIDMappings     []groot.IDMappingSpec
	NamespaceWriter NamespaceWriter
	StoreSizeBytes  int64
}

func New(storePath string, locksmith groot.Locksmith, volumeDriver base_image_puller.VolumeDriver, imageDriver image_cloner.ImageDriver, storeDriver StoreDriver) *Manager {
	return &Manager{
		storePath:    storePath,
		volumeDriver: volumeDriver,
		imageDriver:  imageDriver,
		storeDriver:  storeDriver,
		locksmith:    locksmith,
	}
}

func (m *Manager) InitStore(logger lager.Logger, spec InitSpec) error {
	logger = logger.Session("store-manager-init-store", lager.Data{"storePath": m.storePath})
	logger.Debug("starting")
	defer logger.Debug("ending")

	stat, err := os.Stat(m.storePath)
	if err == nil && stat.IsDir() {
		logger.Error("store-path-already-exists", errorspkg.Errorf("%s already exists", m.storePath))
		return errorspkg.Errorf("store already initialized at path %s", m.storePath)
	}

	validationPath := filepath.Dir(m.storePath)

	if spec.StoreSizeBytes > 0 {
		if spec.StoreSizeBytes < MinStoreSizeBytes {
			logger.Error("init-store-failed", errors.New("store size myst be at least 200Mb"), lager.Data{"storeSize": spec.StoreSizeBytes})
			return errorspkg.New("store size must be at least 200Mb")
		}

		backingStoreFile := fmt.Sprintf("%s.backing-store", m.storePath)
		if _, err := os.Stat(backingStoreFile); err == nil {
			logger.Error("backing-store-file-already-exists", errorspkg.Errorf("%s already exists", backingStoreFile))
			return errorspkg.Errorf("backing store file already exists at path %s", backingStoreFile)
		}

		if err := ioutil.WriteFile(backingStoreFile, []byte{}, 0600); err != nil {
			logger.Error("writing-backingstore-file", err, lager.Data{"backingstoreFile": backingStoreFile})
			return errorspkg.Wrap(err, "creating backing store file")
		}

		if err = os.Truncate(backingStoreFile, spec.StoreSizeBytes); err != nil {
			logger.Error("trunctaing-backing-store-file-failed", err, lager.Data{"backingstoreFile": backingStoreFile, "size": spec.StoreSizeBytes})
			return errorspkg.Wrap(err, "truncating backing store file")
		}

		if err := os.MkdirAll(m.storePath, 0755); err != nil {
			logger.Error("init-store-failed", err)
			return errorspkg.Wrap(err, "initializing store")
		}

		if err := m.storeDriver.InitFilesystem(logger, backingStoreFile, m.storePath); err != nil {
			logger.Error("initializing-filesystem-failed", err, lager.Data{"backingstoreFile": backingStoreFile})
			return errorspkg.Wrap(err, "initializing filesyztem")
		}

		validationPath = m.storePath
	}

	if err := m.storeDriver.ValidateFileSystem(logger, validationPath); err != nil {
		logger.Error("store-path-validation-failed", err)
		return errorspkg.Wrap(err, "validating store path filesystem")
	}

	if err := os.MkdirAll(m.storePath, 0755); err != nil {
		logger.Error("init-store-failed", err)
		return errorspkg.Wrap(err, "initializing store")
	}

	ownerUID, ownerGID := m.findStoreOwner(spec.UIDMappings, spec.GIDMappings)
	if err := os.Chown(m.storePath, ownerUID, ownerGID); err != nil {
		logger.Error("chowning-store-path-failed", err, lager.Data{"uid": ownerUID, "gid": ownerGID})
		return errorspkg.Wrap(err, "chowing store")
	}

	if err := m.storeDriver.ConfigureStore(logger, m.storePath, ownerUID, ownerGID); err != nil {
		logger.Error("store-filesystem-specific-configuration-failed", err)
		return errorspkg.Wrap(err, "running filesystem-specific configuration")
	}

	if err := m.createInternalDirectory(logger, store.MetaDirName, ownerUID, ownerGID); err != nil {
		logger.Error("creating-metadata-dir-failed", err)
		return errorspkg.Wrap(err, "creating metadata dir")
	}

	err = spec.NamespaceWriter.Write(m.storePath, spec.UIDMappings, spec.GIDMappings)
	if err != nil {
		logger.Error("writing-namespace-file-failed", err)
		return errorspkg.Wrapf(err, "writing namespace file for storePath %s", m.storePath)
	}

	return nil
}

func (m *Manager) ConfigureStore(logger lager.Logger, ownerUID, ownerGID int) error {
	logger = logger.Session("store-manager-configure-store", lager.Data{"storePath": m.storePath, "ownerUID": ownerUID, "ownerGID": ownerGID})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := isDirectory(m.storePath); err != nil {
		return err
	}

	if err := os.Setenv("TMPDIR", filepath.Join(m.storePath, store.TempDirName)); err != nil {
		return errorspkg.Wrap(err, "could not set TMPDIR")
	}

	requiredFolders := []string{
		store.ImageDirName,
		store.VolumesDirName,
		store.CacheDirName,
		store.LocksDirName,
		store.MetaDirName,
		store.TempDirName,
		filepath.Join(store.MetaDirName, "dependencies"),
	}

	if _, err := os.Stat(m.storePath); os.IsNotExist(err) {
		if err := os.Mkdir(m.storePath, 0755); err != nil {
			dir, err1 := os.Lstat(m.storePath)
			if err1 != nil || !dir.IsDir() {
				logger.Error("creating-store-path-failed", err)
				return errorspkg.Wrapf(err, "making directory `%s`", m.storePath)
			}
		}

		if err := os.Chown(m.storePath, ownerUID, ownerGID); err != nil {
			logger.Error("store-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
			return errorspkg.Wrapf(err, "changing store owner to %d:%d for path %s", ownerUID, ownerGID, m.storePath)
		}

		if err := os.Chmod(m.storePath, 0700); err != nil {
			logger.Error("store-permission-change-failed", err)
			return errorspkg.Wrapf(err, "changing store permissions %s", m.storePath)
		}
	}

	for _, folderName := range requiredFolders {
		if err := m.createInternalDirectory(logger, folderName, ownerUID, ownerGID); err != nil {
			return err
		}
	}

	if err := m.storeDriver.ConfigureStore(logger, m.storePath, ownerUID, ownerGID); err != nil {
		logger.Error("store-filesystem-specific-configuration-failed", err)
		return errorspkg.Wrap(err, "running filesystem-specific configuration")
	}

	if err := m.storeDriver.ValidateFileSystem(logger, m.storePath); err != nil {
		logger.Error("filesystem-validation-failed", err)
		return errorspkg.Wrap(err, "validating file system")
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
		return errorspkg.Wrap(err, "requesting lock")
	}
	defer m.locksmith.Unlock(fileLock)

	existingImages, err := m.images()
	if err != nil {
		return err
	}

	for _, image := range existingImages {
		if err := m.imageDriver.DestroyImage(logger, image); err != nil {
			logger.Error("destroying-image-failed", err, lager.Data{"image": image})
			return errorspkg.Wrapf(err, "destroying image %s", image)
		}
	}

	existingVolumes, err := m.volumes()
	if err != nil {
		return err
	}

	for _, volume := range existingVolumes {
		if err := m.volumeDriver.DestroyVolume(logger, volume); err != nil {
			logger.Error("destroying-volume-failed", err, lager.Data{"volume": volume})
			return errorspkg.Wrapf(err, "destroying volume %s", volume)
		}
	}

	if err := os.RemoveAll(m.storePath); err != nil {
		logger.Error("deleting-store-path-failed", err, lager.Data{"storePath": m.storePath})
		return errorspkg.Wrapf(err, "deleting store path")
	}

	return nil
}

func (m *Manager) images() ([]string, error) {
	imagesPath := filepath.Join(m.storePath, store.ImageDirName)
	images, err := ioutil.ReadDir(imagesPath)
	if err != nil {
		return nil, errorspkg.Wrap(err, "listing images")
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
		return nil, errorspkg.Wrap(err, "listing volumes")
	}

	volumeIds := []string{}
	for _, file := range volumes {
		volumeIds = append(volumeIds, file.Name())
	}

	return volumeIds, nil
}

func (m *Manager) createInternalDirectory(logger lager.Logger, folderName string, ownerUID, ownerGID int) error {
	requiredPath := filepath.Join(m.storePath, folderName)

	if err := isDirectory(requiredPath); err != nil {
		return err
	}

	if err := os.Mkdir(requiredPath, 0755); err != nil {
		dir, err1 := os.Lstat(requiredPath)
		if err1 != nil || !dir.IsDir() {
			return errorspkg.Wrapf(err, "making directory `%s`", requiredPath)
		}
	}

	if err := os.Chown(requiredPath, ownerUID, ownerGID); err != nil {
		logger.Error("store-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
		return errorspkg.Wrapf(err, "changing store owner to %d:%d for path %s", ownerUID, ownerGID, requiredPath)
	}
	return nil
}

func (m *Manager) findStoreOwner(uidMappings, gidMappings []groot.IDMappingSpec) (int, int) {
	uid := os.Getuid()
	gid := os.Getgid()

	for _, mapping := range uidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			uid = mapping.HostID
			break
		}
		uid = -1
	}

	for _, mapping := range gidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			gid = mapping.HostID
			break
		}
		gid = -1
	}

	return uid, gid
}

func isDirectory(requiredPath string) error {
	if info, err := os.Stat(requiredPath); err == nil {
		if !info.IsDir() {
			return errorspkg.Errorf("path `%s` is not a directory", requiredPath)
		}
	}
	return nil
}

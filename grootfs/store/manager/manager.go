package manager

import (
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
	DeInitFilesystem(logger lager.Logger, storePath string) error
}

type Manager struct {
	storePath       string
	imageDriver     image_cloner.ImageDriver
	volumeDriver    base_image_puller.VolumeDriver
	storeDriver     StoreDriver
	storeNamespacer StoreNamespacer
	locksmith       groot.Locksmith
}

//go:generate counterfeiter . StoreNamespacer
type StoreNamespacer interface {
	ApplyMappings(uidMappings, gidMappings []groot.IDMappingSpec) error
}

type InitSpec struct {
	UIDMappings    []groot.IDMappingSpec
	GIDMappings    []groot.IDMappingSpec
	StoreSizeBytes int64
}

func New(storePath string, storeNamespacer StoreNamespacer, volumeDriver base_image_puller.VolumeDriver, imageDriver image_cloner.ImageDriver, storeDriver StoreDriver, locksmith groot.Locksmith) *Manager {
	return &Manager{
		storePath:       storePath,
		volumeDriver:    volumeDriver,
		imageDriver:     imageDriver,
		storeDriver:     storeDriver,
		storeNamespacer: storeNamespacer,
		locksmith:       locksmith,
	}
}

func (m *Manager) InitStore(logger lager.Logger, spec InitSpec) (err error) {
	logger = logger.Session("store-manager-init-store", lager.Data{"storePath": m.storePath, "spec": spec})
	logger.Debug("starting")
	defer logger.Debug("ending")

	validationPath := filepath.Dir(m.storePath)
	stat, err := os.Stat(m.storePath)
	if err == nil && stat.IsDir() {
		logger.Debug("store-path-already-exists", lager.Data{"StorePath": m.storePath})
		validationPath = m.storePath
	}

	if spec.StoreSizeBytes > 0 {
		validationPath = m.storePath
	}

	lockFile, err := m.locksmith.Lock("init-store")
	if err != nil {
		return errorspkg.Wrap(err, "locking")
	}
	defer func() {
		if unlockErr := m.locksmith.Unlock(lockFile); unlockErr != nil {
			err = errorspkg.Wrap(err, unlockErr.Error())
		}
	}()

	if err := m.storeDriver.ValidateFileSystem(logger, validationPath); err != nil {
		logger.Debug(errorspkg.Wrap(err, "store-could-not-be-validated").Error())
		if spec.StoreSizeBytes <= 0 {
			return errorspkg.Wrap(err, "validating store path filesystem")
		}

		if err := m.createAndMountFilesystem(logger, spec.StoreSizeBytes); err != nil {
			return err
		}
	} else {
		logger.Debug("store-already-initialized")
	}

	if err := os.MkdirAll(filepath.Join(m.storePath, store.MetaDirName), 0755); err != nil {
		logger.Error("init-store-failed", err)
		return errorspkg.Wrap(err, "initializing store")
	}

	err = m.storeNamespacer.ApplyMappings(spec.UIDMappings, spec.GIDMappings)
	if err != nil {
		logger.Error("applying-namespace-mappings-failed", err)
		return err
	}

	ownerUID, ownerGID := m.findStoreOwner(spec.UIDMappings, spec.GIDMappings)
	if err := os.Chown(m.storePath, ownerUID, ownerGID); err != nil {
		logger.Error("chowning-store-path-failed", err, lager.Data{"uid": ownerUID, "gid": ownerGID})
		return errorspkg.Wrap(err, "chowing store")
	}

	if err := m.configureStore(logger, ownerUID, ownerGID); err != nil {
		logger.Error("store-filesystem-specific-configuration-failed", err)
		return errorspkg.Wrap(err, "running filesystem-specific configuration")
	}

	return nil
}

func (m *Manager) IsStoreInitialized(logger lager.Logger) bool {
	for _, folderName := range store.StoreFolders {
		if _, err := os.Stat(filepath.Join(m.storePath, folderName)); os.IsNotExist(err) {
			return false
		}
	}

	if _, err := os.Stat(filepath.Join(m.storePath, store.MetaDirName, groot.NamespaceFilename)); os.IsNotExist(err) {
		return false
	}
	return true
}

func (m *Manager) configureStore(logger lager.Logger, ownerUID, ownerGID int) error {
	logger = logger.Session("store-manager-configure-store", lager.Data{"storePath": m.storePath, "ownerUID": ownerUID, "ownerGID": ownerGID})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if err := isDirectory(m.storePath); err != nil {
		return err
	}

	if err := os.Chown(m.storePath, ownerUID, ownerGID); err != nil {
		logger.Error("store-ownership-change-failed", err, lager.Data{"target-uid": ownerUID, "target-gid": ownerGID})
		return errorspkg.Wrapf(err, "changing store owner to %d:%d for path %s", ownerUID, ownerGID, m.storePath)
	}

	if err := os.Chmod(m.storePath, 0700); err != nil {
		logger.Error("store-permission-change-failed", err)
		return errorspkg.Wrapf(err, "changing store permissions %s", m.storePath)
	}

	for _, folderName := range store.StoreFolders {
		if err := m.createInternalDirectory(logger, folderName, ownerUID, ownerGID); err != nil {
			return err
		}
	}

	if err := m.storeDriver.ConfigureStore(logger, m.storePath, ownerUID, ownerGID); err != nil {
		logger.Error("store-filesystem-specific-configuration-failed", err)
		return errorspkg.Wrap(err, "running filesystem-specific configuration")
	}

	return nil
}

func (m *Manager) DeleteStore(logger lager.Logger) error {
	logger = logger.Session("store-manager-delete-store")
	logger.Debug("starting")
	defer logger.Debug("ending")

	if _, err := os.Stat(m.storePath); os.IsNotExist(err) {
		logger.Info("store-not-found", lager.Data{"storePath": m.storePath})
		return nil
	}

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

	if err := m.storeDriver.DeInitFilesystem(logger, m.storePath); err != nil {
		logger.Error("deinitialising-store-failed", err)
		return errorspkg.Wrap(err, "deinitialising store")
	}

	if err := os.RemoveAll(m.storePath); err != nil {
		logger.Error("deleting-store-path-failed", err, lager.Data{"storePath": m.storePath})
		return errorspkg.Wrapf(err, "deleting store path")
	}

	return nil
}

func (m *Manager) createAndMountFilesystem(logger lager.Logger, storeSizeBytes int64) error {
	if err := os.MkdirAll(m.storePath, 0755); err != nil {
		logger.Error("init-store-failed", err)
		return errorspkg.Wrap(err, "initializing store")
	}

	backingStoreFile := fmt.Sprintf("%s.backing-store", m.storePath)
	if _, err := os.Stat(backingStoreFile); os.IsNotExist(err) {
		if err := ioutil.WriteFile(backingStoreFile, []byte{}, 0600); err != nil {
			logger.Error("writing-backing-store-file", err, lager.Data{"backingstoreFile": backingStoreFile})
			return errorspkg.Wrap(err, "creating backing store file")
		}
	}

	if err := os.Truncate(backingStoreFile, storeSizeBytes); err != nil {
		logger.Error("truncating-backing-store-file-failed", err, lager.Data{"backingstoreFile": backingStoreFile, "size": storeSizeBytes})
		return errorspkg.Wrap(err, "truncating backing store file")
	}

	if err := m.storeDriver.InitFilesystem(logger, backingStoreFile, m.storePath); err != nil {
		logger.Error("initializing-filesystem-failed", err, lager.Data{"backingstoreFile": backingStoreFile})
		return errorspkg.Wrap(err, "initializing filesyztem")
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

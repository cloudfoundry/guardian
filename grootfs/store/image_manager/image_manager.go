package image_manager // import "code.cloudfoundry.org/grootfs/store/image_manager"

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager/v3"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

type ImageDriverSpec struct {
	BaseVolumeIDs      []string
	Mount              bool
	ImagePath          string
	DiskLimit          int64
	ExclusiveDiskLimit bool
	OwnerUID           int
	OwnerGID           int
}

//go:generate counterfeiter . ImageDriver
type ImageDriver interface {
	CreateImage(logger lager.Logger, spec ImageDriverSpec) (groot.MountInfo, error)
	DestroyImage(logger lager.Logger, path string) error
	FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error)
}

type ImageManager struct {
	imageDriver ImageDriver
	storePath   string
}

func NewImageManager(imageDriver ImageDriver, storePath string) *ImageManager {
	return &ImageManager{
		imageDriver: imageDriver,
		storePath:   storePath,
	}
}

func (b *ImageManager) ImageIDs(logger lager.Logger) ([]string, error) {
	images := []string{}

	existingImages, err := os.ReadDir(path.Join(b.storePath, store.ImageDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to read images dir")
	}

	for _, imageInfo := range existingImages {
		images = append(images, imageInfo.Name())
	}

	return images, nil
}

func (b *ImageManager) Create(logger lager.Logger, spec groot.ImageSpec) (groot.ImageInfo, error) {
	logger = logger.Session("making-image", lager.Data{"storePath": b.storePath, "id": spec.ID})
	logger.Info("starting")
	defer logger.Info("ending")

	imagePath := b.imagePath(spec.ID)
	imageRootFSPath := filepath.Join(imagePath, "rootfs")

	var err error
	defer func() {
		if err != nil {
			log := logger.Session("create-failed-cleaning-up", lager.Data{
				"id":    spec.ID,
				"cause": err.Error(),
			})

			log.Info("starting")
			defer log.Info("ending")

			if err = b.imageDriver.DestroyImage(logger, imagePath); err != nil {
				log.Error("destroying-rootfs-image", err)
			}

			if err := os.RemoveAll(imagePath); err != nil {
				log.Error("deleting-image-path", err)
			}
		}
	}()

	if err = os.Mkdir(imagePath, 0700); err != nil {
		return groot.ImageInfo{}, errorspkg.Wrap(err, "making image path")
	}

	imageDriverSpec := ImageDriverSpec{
		BaseVolumeIDs:      spec.BaseVolumeIDs,
		Mount:              spec.Mount,
		ImagePath:          imagePath,
		DiskLimit:          spec.DiskLimit,
		ExclusiveDiskLimit: spec.ExcludeBaseImageFromQuota,
		OwnerUID:           spec.OwnerUID,
		OwnerGID:           spec.OwnerGID,
	}

	var mountInfo groot.MountInfo
	if mountInfo, err = b.imageDriver.CreateImage(logger, imageDriverSpec); err != nil {
		logger.Error("creating-image-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.ImageInfo{}, errorspkg.Wrap(err, "creating image")
	}

	if err := b.setOwnership(spec,
		imagePath,
		imageRootFSPath,
	); err != nil {
		logger.Error("setting-permission-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.ImageInfo{}, err
	}

	imageInfo, err := b.imageInfo(imageRootFSPath, imagePath, spec.BaseImage, mountInfo, spec.Mount)
	if err != nil {
		logger.Error("creating-image-object", err)
		return groot.ImageInfo{}, errorspkg.Wrap(err, "creating image object")
	}

	return imageInfo, nil
}

func (b *ImageManager) Destroy(logger lager.Logger, id string) error {
	logger = logger.Session("deleting-image", lager.Data{"storePath": b.storePath, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		if err != nil {
			return errorspkg.Wrapf(err, "unable to check image: %s", id)
		}
		return errorspkg.Errorf("image not found: %s", id)
	}

	imagePath := b.imagePath(id)
	var volDriverErr error
	if volDriverErr = b.imageDriver.DestroyImage(logger, imagePath); volDriverErr != nil {
		logger.Error("destroying-image-failed", volDriverErr)
	}

	if _, err := os.Stat(imagePath); err == nil {
		logger.Error("deleting-image-dir-failed", err, lager.Data{"volumeDriverError": volDriverErr})
		return fmt.Errorf("deleting image path '%s' failed", imagePath)
	}

	return nil
}

func (b *ImageManager) Exists(id string) (bool, error) {
	imagePath := path.Join(b.storePath, store.ImageDirName, id)
	if _, err := os.Stat(imagePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errorspkg.Wrapf(err, "checking if image `%s` exists", id)
	}

	return true, nil
}

func (b *ImageManager) Stats(logger lager.Logger, id string) (groot.VolumeStats, error) {
	logger = logger.Session("fetching-stats", lager.Data{"id": id})
	logger.Debug("starting")
	defer logger.Debug("ending")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		return groot.VolumeStats{}, errorspkg.Errorf("image not found: %s", id)
	}

	imagePath := b.imagePath(id)

	return b.imageDriver.FetchStats(logger, imagePath)
}

var OpenFile = os.OpenFile

func (b *ImageManager) imageInfo(rootfsPath, imagePath string, baseImage specsv1.Image, mountJson groot.MountInfo, mount bool) (groot.ImageInfo, error) {
	imageInfo := groot.ImageInfo{
		Path:   imagePath,
		Rootfs: rootfsPath,
		Image:  baseImage,
	}

	if !mount {
		imageInfo.Mounts = []groot.MountInfo{mountJson}
	}

	return imageInfo, nil
}

func (b *ImageManager) imagePath(id string) string {
	return path.Join(b.storePath, store.ImageDirName, id)
}

func (b *ImageManager) setOwnership(spec groot.ImageSpec, paths ...string) error {
	if spec.OwnerUID == 0 && spec.OwnerGID == 0 {
		return nil
	}

	for _, path := range paths {
		if err := os.Chown(path, spec.OwnerUID, spec.OwnerGID); err != nil {
			return errorspkg.Wrapf(err, "changing %s ownership to %d:%d", path, spec.OwnerUID, spec.OwnerGID)
		}
	}
	return nil
}

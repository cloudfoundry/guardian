package image_cloner // import "code.cloudfoundry.org/grootfs/store/image_cloner"

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

type ImageDriverSpec struct {
	BaseVolumeIDs      []string
	ImagePath          string
	DiskLimit          int64
	ExclusiveDiskLimit bool
}

//go:generate counterfeiter . ImageDriver
type ImageDriver interface {
	CreateImage(logger lager.Logger, spec ImageDriverSpec) error
	DestroyImage(logger lager.Logger, path string) error
	FetchStats(logger lager.Logger, path string) (groot.VolumeStats, error)
}

type ImageCloner struct {
	imageDriver ImageDriver
	storePath   string
}

func NewImageCloner(imageDriver ImageDriver, storePath string) *ImageCloner {
	return &ImageCloner{
		imageDriver: imageDriver,
		storePath:   storePath,
	}
}

func (b *ImageCloner) ImageIDs(logger lager.Logger) ([]string, error) {
	images := []string{}

	existingImages, err := ioutil.ReadDir(path.Join(b.storePath, store.ImageDirName))
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to read images dir")
	}

	for _, imageInfo := range existingImages {
		images = append(images, imageInfo.Name())
	}

	return images, nil
}

func (b *ImageCloner) Create(logger lager.Logger, spec groot.ImageSpec) (groot.Image, error) {
	logger = logger.Session("making-image", lager.Data{"storePath": b.storePath, "id": spec.ID})
	logger.Info("start")
	defer logger.Info("end")

	var err error
	image := b.createImage(spec.ID)
	defer func() {
		if err != nil {
			log := logger.Session("create-failed-cleaning-up", lager.Data{
				"id":    spec.ID,
				"cause": err.Error(),
			})

			log.Info("start")
			defer log.Info("end")

			if err = b.imageDriver.DestroyImage(logger, image.Path); err != nil {
				log.Error("destroying-rootfs-snapshot", err)
			}

			if err = b.deleteImageDir(image); err != nil {
				log.Error("deleting-image-path", err)
			}
		}
	}()

	if err = os.Mkdir(image.Path, 0700); err != nil {
		return groot.Image{}, errorspkg.Wrap(err, "making image path")
	}

	if err = b.writeBaseImageJSON(logger, image, spec.BaseImage); err != nil {
		logger.Error("writing-image-json-failed", err)
		return groot.Image{}, errorspkg.Wrap(err, "creating image.json")
	}

	imageDriverSpec := ImageDriverSpec{
		BaseVolumeIDs:      spec.BaseVolumeIDs,
		ImagePath:          image.Path,
		DiskLimit:          spec.DiskLimit,
		ExclusiveDiskLimit: spec.ExcludeBaseImageFromQuota,
	}

	if err = b.imageDriver.CreateImage(logger, imageDriverSpec); err != nil {
		logger.Error("creating-image-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.Image{}, errorspkg.Wrap(err, "creating image")
	}

	if err := b.setOwnership(spec,
		image.Path,
		filepath.Join(image.Path, "image.json"),
		image.RootFSPath,
	); err != nil {
		logger.Error("setting-permission-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.Image{}, err
	}

	return image, nil
}

func (b *ImageCloner) Destroy(logger lager.Logger, id string) error {
	logger = logger.Session("deleting-image", lager.Data{"storePath": b.storePath, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		return errorspkg.Errorf("image not found: %s", id)
	}

	image := b.createImage(id)
	if err := b.imageDriver.DestroyImage(logger, image.Path); err != nil {
		return errorspkg.Wrap(err, "destroying snapshot")
	}

	if err := b.deleteImageDir(image); err != nil {
		return errorspkg.Wrap(err, "deleting image path")
	}

	return nil
}

func (b *ImageCloner) Exists(id string) (bool, error) {
	imagePath := path.Join(b.storePath, store.ImageDirName, id)
	if _, err := os.Stat(imagePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errorspkg.Wrapf(err, "checking if image `%s` exists", id)
	}

	return true, nil
}

func (b *ImageCloner) Stats(logger lager.Logger, id string) (groot.VolumeStats, error) {
	logger = logger.Session("fetching-stats", lager.Data{"id": id})
	logger.Info("start")
	defer logger.Info("end")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		return groot.VolumeStats{}, errorspkg.Errorf("image not found: %s", id)
	}

	image := b.createImage(id)

	return b.imageDriver.FetchStats(logger, image.Path)
}

func (b *ImageCloner) deleteImageDir(image groot.Image) error {
	if err := os.RemoveAll(image.Path); err != nil {
		return errorspkg.Wrap(err, "deleting image path")
	}

	return nil
}

var OF = os.OpenFile

func (b *ImageCloner) writeBaseImageJSON(logger lager.Logger, image groot.Image, baseImage specsv1.Image) error {
	logger = logger.Session("writing-image-json")
	logger.Info("start")
	defer logger.Info("end")

	imageJsonPath := filepath.Join(image.Path, "image.json")
	imageJsonFile, err := OF(imageJsonPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	if err = json.NewEncoder(imageJsonFile).Encode(baseImage); err != nil {
		return err
	}

	return nil
}

func (b *ImageCloner) createImage(id string) groot.Image {
	imagePath := path.Join(b.storePath, store.ImageDirName, id)

	return groot.Image{
		Path:       imagePath,
		RootFSPath: filepath.Join(imagePath, "rootfs"),
	}
}

func (b *ImageCloner) setOwnership(spec groot.ImageSpec, paths ...string) error {
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

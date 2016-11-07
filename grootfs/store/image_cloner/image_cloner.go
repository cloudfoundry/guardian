package image_cloner // import "code.cloudfoundry.org/grootfs/store/image_cloner"

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . SnapshotDriver

type SnapshotDriver interface {
	Snapshot(logger lager.Logger, fromPath, toPath string) error
	ApplyDiskLimit(logger lager.Logger, path string, diskLimit int64, exclusive bool) error
	FetchMetrics(logger lager.Logger, path string) (groot.VolumeMetrics, error)
	Destroy(logger lager.Logger, path string) error
}

type ImageCloner struct {
	snapshotDriver SnapshotDriver
	storePath      string
}

func NewImageCloner(snapshotDriver SnapshotDriver, storePath string) *ImageCloner {
	return &ImageCloner{
		snapshotDriver: snapshotDriver,
		storePath:      storePath,
	}
}

func (b *ImageCloner) ImageIDs(logger lager.Logger) ([]string, error) {
	images := []string{}

	existingImages, err := ioutil.ReadDir(path.Join(b.storePath, store.IMAGES_DIR_NAME))
	if err != nil {
		return nil, fmt.Errorf("failed to read images dir: %s", err.Error())
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

			if err = b.snapshotDriver.Destroy(logger, image.RootFSPath); err != nil {
				log.Error("destroying-rootfs-snapshot", err)
			}

			if err = b.deleteImageDir(image); err != nil {
				log.Error("deleting-image-path", err)
			}
		}
	}()

	if err = os.Mkdir(image.Path, 0700); err != nil {
		return groot.Image{}, fmt.Errorf("making image path: %s", err)
	}

	if err = b.writeBaseImageJSON(logger, image, spec.BaseImage); err != nil {
		return groot.Image{}, fmt.Errorf("creating image.json: %s", err)
	}

	if err = b.snapshotDriver.Snapshot(logger, spec.VolumePath, image.RootFSPath); err != nil {
		return groot.Image{}, fmt.Errorf("creating snapshot: %s", err)
	}

	if spec.DiskLimit > 0 {
		if err = b.snapshotDriver.ApplyDiskLimit(logger, image.RootFSPath, spec.DiskLimit, spec.ExcludeBaseImageFromQuota); err != nil {
			return groot.Image{}, fmt.Errorf("applying disk limit: %s", err)
		}
	}

	return image, nil
}

func (b *ImageCloner) Destroy(logger lager.Logger, id string) error {
	logger = logger.Session("deleting-image", lager.Data{"storePath": b.storePath, "id": id})
	logger.Info("start")
	defer logger.Info("end")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		return fmt.Errorf("image not found: %s", id)
	}

	image := b.createImage(id)
	if err := b.snapshotDriver.Destroy(logger, image.RootFSPath); err != nil {
		return fmt.Errorf("destroying snapshot: %s", err)
	}

	if err := b.deleteImageDir(image); err != nil {
		return fmt.Errorf("deleting image path: %s", err)
	}

	return nil
}

func (b *ImageCloner) Exists(id string) (bool, error) {
	imagePath := path.Join(b.storePath, store.IMAGES_DIR_NAME, id)
	if _, err := os.Stat(imagePath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("checking if image `%s` exists: `%s`", id, err)
	}

	return true, nil
}

func (b *ImageCloner) Metrics(logger lager.Logger, id string) (groot.VolumeMetrics, error) {
	logger = logger.Session("fetching-metrics", lager.Data{"id": id})
	logger.Info("start")
	defer logger.Info("end")

	image := b.createImage(id)

	return b.snapshotDriver.FetchMetrics(logger, image.RootFSPath)
}

func (b *ImageCloner) deleteImageDir(image groot.Image) error {
	if err := os.RemoveAll(image.Path); err != nil {
		return fmt.Errorf("deleting image path: %s", err)
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
	imagePath := path.Join(b.storePath, store.IMAGES_DIR_NAME, id)

	return groot.Image{
		Path:       imagePath,
		RootFSPath: filepath.Join(imagePath, "rootfs"),
	}
}

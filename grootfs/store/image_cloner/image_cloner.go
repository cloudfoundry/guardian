package image_cloner // import "code.cloudfoundry.org/grootfs/store/image_cloner"

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/lager"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	errorspkg "github.com/pkg/errors"
)

type ImageInfo struct {
	Rootfs string         `json:"rootfs"`
	Config *specsv1.Image `json:"config,omitempty"`
	Mount  *MountInfo     `json:"mount,omitempty"`
}

type MountInfo struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

type ImageDriverSpec struct {
	BaseVolumeIDs      []string
	SkipMount          bool
	ImagePath          string
	DiskLimit          int64
	ExclusiveDiskLimit bool
}

//go:generate counterfeiter . ImageDriver
type ImageDriver interface {
	CreateImage(logger lager.Logger, spec ImageDriverSpec) (MountInfo, error)
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

			if err = b.deleteImageDir(imagePath); err != nil {
				log.Error("deleting-image-path", err)
			}
		}
	}()

	if err = os.Mkdir(imagePath, 0700); err != nil {
		return groot.Image{}, errorspkg.Wrap(err, "making image path")
	}

	if err = b.writeBaseImageJSON(logger, imagePath, spec.BaseImage); err != nil {
		logger.Error("writing-image-json-failed", err)
		return groot.Image{}, errorspkg.Wrap(err, "creating image.json")
	}

	imageDriverSpec := ImageDriverSpec{
		BaseVolumeIDs:      spec.BaseVolumeIDs,
		SkipMount:          spec.SkipMount,
		ImagePath:          imagePath,
		DiskLimit:          spec.DiskLimit,
		ExclusiveDiskLimit: spec.ExcludeBaseImageFromQuota,
	}

	var mountJson MountInfo
	if mountJson, err = b.imageDriver.CreateImage(logger, imageDriverSpec); err != nil {
		logger.Error("creating-image-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.Image{}, errorspkg.Wrap(err, "creating image")
	}

	if err := b.setOwnership(spec,
		imagePath,
		filepath.Join(imagePath, "image.json"),
		imageRootFSPath,
	); err != nil {
		logger.Error("setting-permission-failed", err, lager.Data{"imageDriverSpec": imageDriverSpec})
		return groot.Image{}, err
	}

	imageJson, err := b.imageJson(imageRootFSPath, spec.BaseImage, mountJson, spec.SkipMount)
	if err != nil {
		logger.Error("creating-image-object", err)
		return groot.Image{}, errorspkg.Wrap(err, "creating image object")
	}

	return groot.Image{
		Path:       imagePath,
		RootFSPath: imageRootFSPath,
		Json:       imageJson,
	}, nil
}

func (b *ImageCloner) Destroy(logger lager.Logger, id string) error {
	logger = logger.Session("deleting-image", lager.Data{"storePath": b.storePath, "id": id})
	logger.Info("starting")
	defer logger.Info("ending")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		return errorspkg.Errorf("image not found: %s", id)
	}

	imagePath := b.imagePath(id)
	if err := b.imageDriver.DestroyImage(logger, imagePath); err != nil {
		return errorspkg.Wrap(err, "destroying image")
	}

	if err := b.deleteImageDir(imagePath); err != nil {
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
	logger.Info("starting")
	defer logger.Info("ending")

	if ok, err := b.Exists(id); !ok {
		logger.Error("checking-image-path-failed", err)
		return groot.VolumeStats{}, errorspkg.Errorf("image not found: %s", id)
	}

	imagePath := b.imagePath(id)

	return b.imageDriver.FetchStats(logger, imagePath)
}

func (b *ImageCloner) deleteImageDir(imagePath string) error {
	if err := os.RemoveAll(imagePath); err != nil {
		return errorspkg.Wrap(err, "deleting image path")
	}

	return nil
}

var OF = os.OpenFile

func (b *ImageCloner) writeBaseImageJSON(logger lager.Logger, imagePath string, baseImage specsv1.Image) error {
	logger = logger.Session("writing-image-json")
	logger.Info("starting")
	defer logger.Info("ending")

	imageJsonPath := filepath.Join(imagePath, "image.json")
	imageJsonFile, err := OF(imageJsonPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}

	if err = json.NewEncoder(imageJsonFile).Encode(baseImage); err != nil {
		return err
	}

	return nil
}

func (b *ImageCloner) imageJson(rootfsPath string, baseImage specsv1.Image, mountJson MountInfo, skipMount bool) (string, error) {
	var imageConfig *specsv1.Image
	if !reflect.DeepEqual(baseImage, specsv1.Image{}) {
		imageConfig = &baseImage
	}

	imageJson := ImageInfo{
		Rootfs: rootfsPath,
		Config: imageConfig,
	}

	if skipMount {
		imageJson.Mount = &mountJson
	}

	jsonBytes, err := json.Marshal(&imageJson)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

func (b *ImageCloner) imagePath(id string) string {
	return path.Join(b.storePath, store.ImageDirName, id)
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

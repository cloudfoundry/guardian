package groot

import (
	"fmt"
	"net/url"
	"strings"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const ImageReferenceFormat = "image:%s"

type CreateSpec struct {
	ID                        string
	BaseImage                 string
	DiskLimit                 int64
	ExcludeBaseImageFromQuota bool
	UIDMappings               []IDMappingSpec
	GIDMappings               []IDMappingSpec
}

type Creator struct {
	imageCloner       ImageCloner
	baseImagePuller   BaseImagePuller
	locksmith         Locksmith
	rootFSConfigurer  RootFSConfigurer
	dependencyManager DependencyManager
}

func IamCreator(imageCloner ImageCloner, baseImagePuller BaseImagePuller, locksmith Locksmith, rootFSConfigurer RootFSConfigurer, dependencyManager DependencyManager) *Creator {
	return &Creator{
		imageCloner:       imageCloner,
		baseImagePuller:   baseImagePuller,
		locksmith:         locksmith,
		rootFSConfigurer:  rootFSConfigurer,
		dependencyManager: dependencyManager,
	}
}

func (c *Creator) Create(logger lager.Logger, spec CreateSpec) (Image, error) {
	logger = logger.Session("groot-creating", lager.Data{"imageID": spec.ID, "spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	parsedURL, err := url.Parse(spec.BaseImage)
	if err != nil {
		return Image{}, fmt.Errorf("parsing image url: %s", err)
	}

	if strings.ContainsAny(spec.ID, "/") {
		return Image{}, fmt.Errorf("id `%s` contains invalid characters: `/`", spec.ID)
	}

	ok, err := c.imageCloner.Exists(spec.ID)
	if err != nil {
		return Image{}, fmt.Errorf("checking id exists: %s", err)
	}
	if ok {
		return Image{}, fmt.Errorf("image for id `%s` already exists", spec.ID)
	}

	baseImageSpec := BaseImageSpec{
		BaseImageSrc:              parsedURL,
		DiskLimit:                 spec.DiskLimit,
		ExcludeBaseImageFromQuota: spec.ExcludeBaseImageFromQuota,
		UIDMappings:               spec.UIDMappings,
		GIDMappings:               spec.GIDMappings,
	}

	lockFile, err := c.locksmith.Lock(GlobalLockKey)
	if err != nil {
		return Image{}, err
	}
	defer func() {
		if err := c.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	baseImage, err := c.baseImagePuller.Pull(logger, baseImageSpec)
	if err != nil {
		return Image{}, errorspkg.Wrap(err, "pulling the image")
	}

	imageSpec := ImageSpec{
		ID:                        spec.ID,
		DiskLimit:                 spec.DiskLimit,
		ExcludeBaseImageFromQuota: spec.ExcludeBaseImageFromQuota,
		VolumePath:                baseImage.VolumePath,
		BaseImage:                 baseImage.BaseImage,
	}
	image, err := c.imageCloner.Create(logger, imageSpec)
	if err != nil {
		return Image{}, fmt.Errorf("making image: %s", err)
	}

	imageRefName := fmt.Sprintf(ImageReferenceFormat, spec.ID)
	if err := c.dependencyManager.Register(imageRefName, baseImage.ChainIDs); err != nil {
		if destroyErr := c.imageCloner.Destroy(logger, spec.ID); destroyErr != nil {
			logger.Error("failed-to-destroy-image", destroyErr)
		}

		return Image{}, err
	}

	if err := c.rootFSConfigurer.Configure(image.RootFSPath, baseImage.BaseImage); err != nil {
		if destroyErr := c.imageCloner.Destroy(logger, spec.ID); destroyErr != nil {
			logger.Error("failed-to-destroy-image", destroyErr)
		}

		return Image{}, err
	}

	return image, nil
}

package groot

import (
	"fmt"
	"net/url"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const ImageReferenceFormat = "image:%s"
const BundleReferenceFormat = "bundle:%s"

type CreateSpec struct {
	ID                        string
	BaseImage                 string
	DiskLimit                 int64
	ExcludeBaseImageFromQuota bool
	UIDMappings               []IDMappingSpec
	GIDMappings               []IDMappingSpec
}

type Creator struct {
	bundler           Bundler
	baseImagePuller   BaseImagePuller
	locksmith         Locksmith
	dependencyManager DependencyManager
}

func IamCreator(bundler Bundler, baseImagePuller BaseImagePuller, locksmith Locksmith, dependencyManager DependencyManager) *Creator {
	return &Creator{
		bundler:           bundler,
		baseImagePuller:   baseImagePuller,
		locksmith:         locksmith,
		dependencyManager: dependencyManager,
	}
}

func (c *Creator) Create(logger lager.Logger, spec CreateSpec) (Bundle, error) {
	logger = logger.Session("groot-creating", lager.Data{"bundleID": spec.ID, "spec": spec})
	logger.Info("start")
	defer logger.Info("end")

	parsedURL, err := url.Parse(spec.BaseImage)
	if err != nil {
		return Bundle{}, fmt.Errorf("parsing image url: %s", err)
	}

	ok, err := c.bundler.Exists(spec.ID)
	if err != nil {
		return Bundle{}, fmt.Errorf("checking id exists: %s", err)
	}
	if ok {
		return Bundle{}, fmt.Errorf("bundle for id `%s` already exists", spec.ID)
	}

	imageSpec := BaseImageSpec{
		BaseImageSrc:              parsedURL,
		DiskLimit:                 spec.DiskLimit,
		ExcludeBaseImageFromQuota: spec.ExcludeBaseImageFromQuota,
		UIDMappings:               spec.UIDMappings,
		GIDMappings:               spec.GIDMappings,
	}

	lockFile, err := c.locksmith.Lock(GLOBAL_LOCK_KEY)
	if err != nil {
		return Bundle{}, err
	}
	defer func() {
		if err := c.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	baseImage, err := c.baseImagePuller.Pull(logger, imageSpec)
	if err != nil {
		return Bundle{}, errorspkg.Wrap(err, "pulling the image")
	}

	bundleSpec := BundleSpec{
		ID:                        spec.ID,
		DiskLimit:                 spec.DiskLimit,
		ExcludeBaseImageFromQuota: spec.ExcludeBaseImageFromQuota,
		VolumePath:                baseImage.VolumePath,
		BaseImage:                 baseImage.BaseImage,
	}
	bundle, err := c.bundler.Create(logger, bundleSpec)
	if err != nil {
		return Bundle{}, fmt.Errorf("making bundle: %s", err)
	}

	bundleRefName := fmt.Sprintf(BundleReferenceFormat, spec.ID)
	if err := c.dependencyManager.Register(bundleRefName, baseImage.ChainIDs); err != nil {
		if destroyErr := c.bundler.Destroy(logger, spec.ID); destroyErr != nil {
			logger.Error("failed-to-destroy-bundle", destroyErr)
		}

		return Bundle{}, err
	}

	imageRefName := fmt.Sprintf(ImageReferenceFormat, spec.BaseImage)
	if err := c.dependencyManager.Register(imageRefName, baseImage.ChainIDs); err != nil {
		return Bundle{}, err
	}

	return bundle, nil
}

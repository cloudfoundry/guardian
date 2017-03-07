package groot

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

const ImageReferenceFormat = "image:%s"

type CreateSpec struct {
	ID                          string
	BaseImage                   string
	DiskLimit                   int64
	ExcludeBaseImageFromQuota   bool
	CleanOnCreate               bool
	CleanOnCreateThresholdBytes int64
	CleanOnCreateIgnoreImages   []string
	UIDMappings                 []IDMappingSpec
	GIDMappings                 []IDMappingSpec
}

type Creator struct {
	cleaner           Cleaner
	imageCloner       ImageCloner
	baseImagePuller   BaseImagePuller
	locksmith         Locksmith
	rootFSConfigurer  RootFSConfigurer
	dependencyManager DependencyManager
	metricsEmitter    MetricsEmitter
	namespaceChecker  NamespaceChecker
}

func IamCreator(
	imageCloner ImageCloner, baseImagePuller BaseImagePuller,
	locksmith Locksmith, rootFSConfigurer RootFSConfigurer,
	dependencyManager DependencyManager, metricsEmitter MetricsEmitter, cleaner Cleaner,
	namespaceChecker NamespaceChecker) *Creator {
	return &Creator{
		imageCloner:       imageCloner,
		baseImagePuller:   baseImagePuller,
		locksmith:         locksmith,
		rootFSConfigurer:  rootFSConfigurer,
		dependencyManager: dependencyManager,
		metricsEmitter:    metricsEmitter,
		cleaner:           cleaner,
		namespaceChecker:  namespaceChecker,
	}
}

func (c *Creator) Create(logger lager.Logger, spec CreateSpec) (Image, error) {
	startTime := time.Now()
	defer func() {
		c.metricsEmitter.TryEmitDuration(logger, MetricImageCreationTime, time.Since(startTime))
	}()

	logger = logger.Session("groot-creating", lager.Data{"imageID": spec.ID, "spec": spec})
	logger.Info("starting")
	defer logger.Info("ending")

	parsedURL, err := url.Parse(spec.BaseImage)
	if err != nil {
		return Image{}, errorspkg.Wrap(err, "parsing image url")
	}

	if strings.ContainsAny(spec.ID, "/") {
		return Image{}, errorspkg.Errorf("id `%s` contains invalid characters: `/`", spec.ID)
	}

	ok, err := c.imageCloner.Exists(spec.ID)
	if err != nil {
		return Image{}, errorspkg.Wrap(err, "checking id exists")
	}
	if ok {
		return Image{}, errorspkg.Errorf("image for id `%s` already exists", spec.ID)
	}

	ownerUid, ownerGid := c.parseOwner(spec.UIDMappings, spec.GIDMappings)
	baseImageSpec := BaseImageSpec{
		BaseImageSrc:              parsedURL,
		DiskLimit:                 spec.DiskLimit,
		ExcludeBaseImageFromQuota: spec.ExcludeBaseImageFromQuota,
		UIDMappings:               spec.UIDMappings,
		GIDMappings:               spec.GIDMappings,
		OwnerUID:                  ownerUid,
		OwnerGID:                  ownerGid,
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

	validNamespace, err := c.namespaceChecker.Check(spec.UIDMappings, spec.GIDMappings)
	if err != nil {
		logger.Error("failed-check-namespace", err)
		return Image{}, errorspkg.Wrap(err, "checking namespace failed")
	}

	if !validNamespace {
		logger.Error("failed-check-namespace", err)
		return Image{}, errorspkg.New("store already initialized with a different mapping")
	}

	if spec.CleanOnCreate {
		ignoredImages := append(spec.CleanOnCreateIgnoreImages, spec.BaseImage)
		if _, err := c.cleaner.Clean(logger, spec.CleanOnCreateThresholdBytes, ignoredImages, false); err != nil {
			return Image{}, errorspkg.Wrap(err, "failed-to-cleanup-store")
		}
	}

	baseImage, err := c.baseImagePuller.Pull(logger, baseImageSpec)
	if err != nil {
		return Image{}, errorspkg.Wrap(err, "pulling the image")
	}

	imageSpec := ImageSpec{
		ID:                        spec.ID,
		DiskLimit:                 spec.DiskLimit,
		ExcludeBaseImageFromQuota: spec.ExcludeBaseImageFromQuota,
		BaseVolumeIDs:             baseImage.ChainIDs,
		BaseImage:                 baseImage.BaseImage,
		OwnerUID:                  ownerUid,
		OwnerGID:                  ownerGid,
	}
	image, err := c.imageCloner.Create(logger, imageSpec)
	if err != nil {
		return Image{}, errorspkg.Wrap(err, "making image")
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

func (c *Creator) parseOwner(uidMappings, gidMappings []IDMappingSpec) (int, int) {
	uid := os.Getuid()
	gid := os.Getgid()

	for _, mapping := range uidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			uid = mapping.HostID
			break
		}
	}

	for _, mapping := range gidMappings {
		if mapping.Size == 1 && mapping.NamespaceID == 0 {
			gid = mapping.HostID
			break
		}
	}

	return uid, gid
}

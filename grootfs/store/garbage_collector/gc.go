package garbage_collector

import (
	"fmt"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . CacheDriver
//go:generate counterfeiter . ImageCloner
//go:generate counterfeiter . DependencyManager
//go:generate counterfeiter . VolumeDriver

type CacheDriver interface {
	Clean(logger lager.Logger) error
}

type ImageCloner interface {
	ImageIDs(logger lager.Logger) ([]string, error)
}

type DependencyManager interface {
	Dependencies(id string) ([]string, error)
}

type VolumeDriver interface {
	DestroyVolume(logger lager.Logger, id string) error
	Volumes(logger lager.Logger) ([]string, error)
}

type GarbageCollector struct {
	cacheDriver       CacheDriver
	volumeDriver      VolumeDriver
	imageCloner       ImageCloner
	dependencyManager DependencyManager
}

func NewGC(cacheDriver CacheDriver, volumeDriver VolumeDriver, imageCloner ImageCloner, dependencyManager DependencyManager) *GarbageCollector {
	return &GarbageCollector{
		cacheDriver:       cacheDriver,
		volumeDriver:      volumeDriver,
		imageCloner:       imageCloner,
		dependencyManager: dependencyManager,
	}
}

func (g *GarbageCollector) Collect(logger lager.Logger, keepImages []string) error {
	logger = logger.Session("garbage-collector-collect")
	logger.Info("starting")
	defer logger.Info("ending")

	if err := g.collectVolumes(logger, keepImages); err != nil {
		return err
	}

	return g.collectBlobs(logger)
}

func (g *GarbageCollector) collectVolumes(logger lager.Logger, keepImages []string) error {
	logger = logger.Session("collect-volumes")
	logger.Info("starting")
	defer logger.Info("ending")

	unusedVolumes, err := g.unusedVolumes(logger, keepImages)
	if err != nil {
		return errorspkg.Wrap(err, "listing volumes")
	}
	logger.Debug("unused-volumes", lager.Data{"unusedVolumes": unusedVolumes})

	var cleanupErr error
	for volID, _ := range unusedVolumes {
		if err := g.volumeDriver.DestroyVolume(logger, volID); err != nil {
			logger.Error("failed-to-destroy-volume", err, lager.Data{"volumeID": volID})
			cleanupErr = errorspkg.New("destroying volumes failed")
		}
	}

	return cleanupErr
}

func (g *GarbageCollector) collectBlobs(logger lager.Logger) error {
	logger = logger.Session("collect-blobs")
	logger.Info("starting")
	defer logger.Info("ending")

	return g.cacheDriver.Clean(logger)
}

func (g *GarbageCollector) unusedVolumes(logger lager.Logger, keepImages []string) (map[string]bool, error) {
	logger = logger.Session("unused-volumes")
	logger.Info("starting")
	defer logger.Info("ending")

	volumes, err := g.volumeDriver.Volumes(logger)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to retrieve volume list")
	}

	orphanedVolumes := make(map[string]bool)
	for _, vol := range volumes {
		orphanedVolumes[vol] = true
	}

	imageIDs, err := g.imageCloner.ImageIDs(logger)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to retrieve images")
	}

	for _, imageID := range imageIDs {
		imageRefName := fmt.Sprintf(groot.ImageReferenceFormat, imageID)
		if err := g.removeDependencies(orphanedVolumes, imageRefName); err != nil {
			return nil, err
		}
	}

	for _, keepImage := range keepImages {
		imageRefName := fmt.Sprintf(base_image_puller.BaseImageReferenceFormat, keepImage)
		if err := g.removeDependencies(orphanedVolumes, imageRefName); err != nil {
			logger.Error("failed-to-find-white-listed-image-dependencies", err, lager.Data{"imageRefName": imageRefName})
		}
	}

	return orphanedVolumes, nil
}

func (g *GarbageCollector) removeDependencies(volumesList map[string]bool, refId string) error {
	usedVolumes, err := g.dependencyManager.Dependencies(refId)
	if err != nil {
		return err
	}

	for _, volumeID := range usedVolumes {
		delete(volumesList, volumeID)
	}

	return nil
}

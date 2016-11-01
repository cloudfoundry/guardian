package garbage_collector

import (
	"errors"
	"fmt"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . CacheDriver
type CacheDriver interface {
	Clean(logger lager.Logger) error
}

//go:generate counterfeiter . VolumeDriver
type VolumeDriver interface {
	DestroyVolume(logger lager.Logger, id string) error
	Volumes(logger lager.Logger) ([]string, error)
}

//go:generate counterfeiter . Bundler
type Bundler interface {
	BundleIDs(logger lager.Logger) ([]string, error)
}

//go:generate counterfeiter . DependencyManager
type DependencyManager interface {
	Dependencies(id string) ([]string, error)
}

type GarbageCollector struct {
	cacheDriver       CacheDriver
	volumeDriver      VolumeDriver
	bundler           Bundler
	dependencyManager DependencyManager
}

func NewGC(cacheDriver CacheDriver, volumeDriver VolumeDriver, bundler Bundler, dependencyManager DependencyManager) *GarbageCollector {
	return &GarbageCollector{
		cacheDriver:       cacheDriver,
		volumeDriver:      volumeDriver,
		bundler:           bundler,
		dependencyManager: dependencyManager,
	}
}

func (g *GarbageCollector) Collect(logger lager.Logger, keepImages []string) error {
	logger = logger.Session("garbage-collector-collect")
	logger.Info("start")
	defer logger.Info("end")

	if err := g.collectVolumes(logger, keepImages); err != nil {
		return err
	}

	return g.collectBlobs(logger)
}

func (g *GarbageCollector) collectVolumes(logger lager.Logger, keepImages []string) error {
	logger = logger.Session("collect-volumes")
	logger.Info("start")
	defer logger.Info("end")

	unusedVolumes, err := g.unusedVolumes(logger, keepImages)
	if err != nil {
		return fmt.Errorf("listing volumes: %s", err.Error())
	}
	logger.Debug("unused-volumes", lager.Data{"unusedVolumes": unusedVolumes})

	var cleanupErr error
	for volID, _ := range unusedVolumes {
		if err := g.volumeDriver.DestroyVolume(logger, volID); err != nil {
			logger.Error("failed-to-destroy-volume", err, lager.Data{"volumeID": volID})
			cleanupErr = errors.New("destroying volumes failed")
		}
	}

	return cleanupErr
}

func (g *GarbageCollector) collectBlobs(logger lager.Logger) error {
	logger = logger.Session("collect-blobs")
	logger.Info("start")
	defer logger.Info("end")

	return g.cacheDriver.Clean(logger)
}

func (g *GarbageCollector) unusedVolumes(logger lager.Logger, keepImages []string) (map[string]bool, error) {
	logger = logger.Session("unused-volumes")
	logger.Info("start")
	defer logger.Info("end")

	volumes, err := g.volumeDriver.Volumes(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve volume list")
	}

	orphanedVolumes := make(map[string]bool)
	for _, vol := range volumes {
		orphanedVolumes[vol] = true
	}

	bundleIDs, err := g.bundler.BundleIDs(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve bundles: %s", err.Error())
	}

	for _, bundleID := range bundleIDs {
		bundleRefName := fmt.Sprintf(groot.BundleReferenceFormat, bundleID)
		if err := g.removeDependencies(orphanedVolumes, bundleRefName); err != nil {
			return nil, err
		}
	}

	for _, keepImage := range keepImages {
		imageRefName := fmt.Sprintf(groot.ImageReferenceFormat, keepImage)
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

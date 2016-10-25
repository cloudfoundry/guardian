package garbage_collector

import (
	"errors"
	"fmt"

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

func (g *GarbageCollector) Collect(logger lager.Logger) error {
	logger = logger.Session("garbage-collector-collect")
	logger.Info("start")
	defer logger.Info("end")

	if err := g.collectVolumes(logger); err != nil {
		return err
	}

	return g.collectBlobs(logger)
}

func (g *GarbageCollector) collectVolumes(logger lager.Logger) error {
	logger = logger.Session("collect-volumes")
	logger.Info("start")
	defer logger.Info("end")

	unusedVolumes, err := g.unusedVolumes(logger)
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

func (g *GarbageCollector) unusedVolumes(logger lager.Logger) (map[string]bool, error) {
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
		usedVolumes, err := g.dependencyManager.Dependencies(bundleID)
		if err != nil {
			return nil, err
		}

		for _, volumeID := range usedVolumes {
			delete(orphanedVolumes, volumeID)
		}
	}

	return orphanedVolumes, nil
}

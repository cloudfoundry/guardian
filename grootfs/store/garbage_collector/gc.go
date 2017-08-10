package garbage_collector

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . ImageCloner
//go:generate counterfeiter . DependencyManager
//go:generate counterfeiter . VolumeDriver

type ImageCloner interface {
	ImageIDs(logger lager.Logger) ([]string, error)
}

type DependencyManager interface {
	Dependencies(id string) ([]string, error)
}

type VolumeDriver interface {
	VolumePath(logger lager.Logger, id string) (string, error)
	MoveVolume(logger lager.Logger, from, to string) error
	DestroyVolume(logger lager.Logger, id string) error
	Volumes(logger lager.Logger) ([]string, error)
}

type GarbageCollector struct {
	volumeDriver      VolumeDriver
	imageCloner       ImageCloner
	dependencyManager DependencyManager
}

func NewGC(volumeDriver VolumeDriver, imageCloner ImageCloner, dependencyManager DependencyManager) *GarbageCollector {
	return &GarbageCollector{
		volumeDriver:      volumeDriver,
		imageCloner:       imageCloner,
		dependencyManager: dependencyManager,
	}
}

func (g *GarbageCollector) MarkUnused(logger lager.Logger, keepImages []string) error {
	logger = logger.Session("garbage-collector-mark-unused")
	logger.Info("starting")
	defer logger.Info("ending")

	unusedVolumes, err := g.unusedVolumes(logger, keepImages)
	if err != nil {
		return errorspkg.Wrap(err, "listing volumes")
	}

	logger.Debug("unused-volumes", lager.Data{"unusedVolumes": unusedVolumes})

	var errorMessages []string
	totalUnusedVolumes := len(unusedVolumes)

	for volID, _ := range unusedVolumes {
		volumePath, err := g.volumeDriver.VolumePath(logger, volID)
		if err != nil {
			errorMessages = append(errorMessages, errorspkg.Wrap(err, "fetching-volume-path").Error())
			continue
		}

		gcVolID := fmt.Sprintf("gc.%s.%d", volID, time.Now().UnixNano())
		gcVolumePath := strings.Replace(volumePath, volID, gcVolID, 1)
		if err := g.volumeDriver.MoveVolume(logger, volumePath, gcVolumePath); err != nil {
			errorMessages = append(errorMessages, errorspkg.Wrap(err, "moving-volume").Error())
		}
	}

	if len(errorMessages) > 0 {
		logger.Error("marking-unused-failed", errors.New("not all volumes were marked"), lager.Data{
			"errors":             errorMessages,
			"totalUnusedVolumes": totalUnusedVolumes,
			"totalFailedVolumes": len(errorMessages),
			"totalVolumesMoved":  totalUnusedVolumes - len(errorMessages),
		})
		return errorspkg.Errorf("%d/%d volumes failed to be marked as unused", len(errorMessages), totalUnusedVolumes)
	}

	return nil
}

func (g *GarbageCollector) Collect(logger lager.Logger) error {
	logger = logger.Session("garbage-collector-collect")
	logger.Info("starting")
	defer logger.Info("ending")

	return g.collectVolumes(logger)
}

func (g *GarbageCollector) collectVolumes(logger lager.Logger) error {
	logger = logger.Session("collect-volumes")
	logger.Info("starting")
	defer logger.Info("ending")

	unusedVolumes, err := g.gcVolumes(logger)
	if err != nil {
		return errorspkg.Wrap(err, "listing volumes")
	}

	var cleanupErr error
	for volID, _ := range unusedVolumes {
		if !strings.HasPrefix(volID, "gc.") {
			continue
		}

		if err := g.volumeDriver.DestroyVolume(logger, volID); err != nil {
			logger.Error("failed-to-destroy-volume", err, lager.Data{"volumeID": volID})
			cleanupErr = errorspkg.New("destroying volumes failed")
		}
	}

	return cleanupErr
}

func (g *GarbageCollector) gcVolumes(logger lager.Logger) (map[string]bool, error) {
	logger = logger.Session("unused-volumes")
	logger.Info("starting")
	defer logger.Info("ending")

	volumes, err := g.volumeDriver.Volumes(logger)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to retrieve volume list")
	}

	collectables := map[string]bool{}
	for _, vol := range volumes {
		if strings.HasPrefix(vol, "gc.") {
			collectables[vol] = true
		}
	}

	return collectables, nil
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
		if !strings.HasPrefix(vol, "gc.") {
			orphanedVolumes[vol] = true
		}
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

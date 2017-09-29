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
	baseImage         string
}

func NewGC(volumeDriver VolumeDriver, imageCloner ImageCloner, dependencyManager DependencyManager, baseImage string) *GarbageCollector {
	return &GarbageCollector{
		volumeDriver:      volumeDriver,
		imageCloner:       imageCloner,
		dependencyManager: dependencyManager,
		baseImage:         baseImage,
	}
}

func (g *GarbageCollector) MarkUnused(logger lager.Logger, unusedVolumes []string) error {
	logger = logger.Session("garbage-collector-mark-unused", lager.Data{"unusedVolumes": unusedVolumes})
	logger.Info("starting")
	defer logger.Info("ending")

	var errorMessages []string
	totalUnusedVolumes := len(unusedVolumes)

	for _, volID := range unusedVolumes {
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
	for _, volID := range unusedVolumes {
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

func (g *GarbageCollector) gcVolumes(logger lager.Logger) ([]string, error) {
	logger = logger.Session("unused-volumes")
	logger.Info("starting")
	defer logger.Info("ending")

	volumes, err := g.volumeDriver.Volumes(logger)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to retrieve volume list")
	}

	collectables := []string{}
	for _, vol := range volumes {
		if strings.HasPrefix(vol, "gc.") {
			collectables = append(collectables, vol)
		}
	}

	return collectables, nil
}

func (g *GarbageCollector) UnusedVolumes(logger lager.Logger) ([]string, error) {
	logger = logger.Session("unused-volumes")
	logger.Info("starting")
	defer logger.Info("ending")

	volumes, err := g.volumeDriver.Volumes(logger)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to retrieve volume list")
	}

	orphanedVolumes := make(map[string]struct{})
	for _, vol := range volumes {
		if !strings.HasPrefix(vol, "gc.") {
			orphanedVolumes[vol] = struct{}{}
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

	if g.baseImage != "" {
		imageRefName := fmt.Sprintf(base_image_puller.BaseImageReferenceFormat, g.baseImage)
		if err := g.removeDependencies(orphanedVolumes, imageRefName); err != nil {
			logger.Error("failed-to-find-base-image-dependencies", err, lager.Data{"imageRefName": imageRefName})
		}
	}

	orphanedVolumeIDs := []string{}
	for id := range orphanedVolumes {
		orphanedVolumeIDs = append(orphanedVolumeIDs, id)
	}
	return orphanedVolumeIDs, nil
}

func (g *GarbageCollector) removeDependencies(volumesList map[string]struct{}, refID string) error {
	usedVolumes, err := g.dependencyManager.Dependencies(refID)
	if err != nil {
		return err
	}

	for _, volumeID := range usedVolumes {
		delete(volumesList, volumeID)
	}

	return nil
}

package garbage_collector

import (
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager/v3"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . ImageIDsGetter
//go:generate counterfeiter . DependencyManager
//go:generate counterfeiter . VolumeDriver

type ImageIDsGetter interface {
	ImageIDs(logger lager.Logger) ([]string, error)
}

type DependencyManager interface {
	Dependencies(id string) ([]string, error)
}

type VolumeDriver interface {
	VolumePath(logger lager.Logger, id string) (string, error)
	MoveVolume(logger lager.Logger, from, to string) error
	DestroyVolume(logger lager.Logger, id string) error
	MarkVolumeArtifacts(logger lager.Logger, id string) error
	Volumes(logger lager.Logger) ([]string, error)
}

type GarbageCollector struct {
	volumeDriver      VolumeDriver
	imageIDsGetter    ImageIDsGetter
	dependencyManager DependencyManager
}

func NewGC(volumeDriver VolumeDriver, imageIDsGetter ImageIDsGetter, dependencyManager DependencyManager) *GarbageCollector {
	return &GarbageCollector{
		volumeDriver:      volumeDriver,
		imageIDsGetter:    imageIDsGetter,
		dependencyManager: dependencyManager,
	}
}

func (g *GarbageCollector) MarkUnused(logger lager.Logger, unusedVolumes []string) error {
	logger = logger.Session("garbage-collector-mark-unused", lager.Data{"unusedVolumes": unusedVolumes})
	logger.Info("starting")
	defer logger.Info("ending")

	var errorMessages []string
	totalUnusedVolumes := len(unusedVolumes)

	for _, volID := range unusedVolumes {
		if err := g.volumeDriver.MarkVolumeArtifacts(logger, volID); err != nil {
			errorMessages = append(errorMessages, errorspkg.Wrap(err, "mark-volume-unused").Error())
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

	imageIDs, err := g.imageIDsGetter.ImageIDs(logger)
	if err != nil {
		return nil, errorspkg.Wrap(err, "failed to retrieve images")
	}

	for _, imageID := range imageIDs {
		imageRefName := fmt.Sprintf(groot.ImageReferenceFormat, imageID)
		usedVolumes, err := g.dependencyManager.Dependencies(imageRefName)
		if err != nil {
			return nil, err
		}
		g.removeDependencyFromOrphanList(orphanedVolumes, usedVolumes)
	}

	orphanedVolumeIDs := []string{}
	for id := range orphanedVolumes {
		orphanedVolumeIDs = append(orphanedVolumeIDs, id)
	}
	return orphanedVolumeIDs, nil
}

func (g *GarbageCollector) removeDependencyFromOrphanList(volumesList map[string]struct{}, usedVolumes []string) {
	for _, volumeID := range usedVolumes {
		delete(volumesList, volumeID)
	}
}

package groot

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
)

type Deleter struct {
	imageCloner       ImageCloner
	dependencyManager DependencyManager
	metricsEmitter    MetricsEmitter
}

func IamDeleter(imageCloner ImageCloner, dependencyManager DependencyManager, metricsEmitter MetricsEmitter) *Deleter {
	return &Deleter{
		imageCloner:       imageCloner,
		dependencyManager: dependencyManager,
		metricsEmitter:    metricsEmitter,
	}
}

func (d *Deleter) Delete(logger lager.Logger, id string) error {
	startTime := time.Now()
	defer func() {
		d.metricsEmitter.TryEmitDuration(logger, MetricImageDeletionTime, time.Since(startTime))
	}()

	logger = logger.Session("groot-deleting", lager.Data{"imageID": id})
	logger.Info("start")
	defer logger.Info("end")

	err := d.imageCloner.Destroy(logger, id)

	imageRefName := fmt.Sprintf(ImageReferenceFormat, id)
	if derErr := d.dependencyManager.Deregister(imageRefName); derErr != nil {
		logger.Error("failed-to-deregister-dependencies", derErr)
	}

	return err
}

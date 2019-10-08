package groot

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

	"code.cloudfoundry.org/lager"
)

type Deleter struct {
	imageManager      ImageManager
	dependencyManager DependencyManager
	metricsEmitter    MetricsEmitter
}

func IamDeleter(imageManager ImageManager, dependencyManager DependencyManager, metricsEmitter MetricsEmitter) *Deleter {
	return &Deleter{
		imageManager:      imageManager,
		dependencyManager: dependencyManager,
		metricsEmitter:    metricsEmitter,
	}
}

func (d *Deleter) Delete(logger lager.Logger, id string) error {
	defer d.metricsEmitter.TryEmitDurationFrom(logger, MetricImageDeletionTime, time.Now())

	logger = logger.Session("groot-deleting", lager.Data{"imageID": id})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := d.imageManager.Destroy(logger, id); err != nil {
		return err
	}

	imageRefName := fmt.Sprintf(ImageReferenceFormat, id)
	if err := d.dependencyManager.Deregister(imageRefName); err != nil {
		if !os.IsNotExist(errors.Cause(err)) {
			logger.Error("failed-to-deregister-dependencies", err)
			return err
		}
	}

	return nil
}

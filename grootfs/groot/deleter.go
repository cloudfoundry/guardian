package groot

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"

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
	defer d.metricsEmitter.TryEmitDurationFrom(logger, MetricImageDeletionTime, time.Now())

	logger = logger.Session("groot-deleting", lager.Data{"imageID": id})
	logger.Info("starting")
	defer logger.Info("ending")

	if err := d.imageCloner.Destroy(logger, id); err != nil {
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

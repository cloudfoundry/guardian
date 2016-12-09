package groot

import (
	"time"

	"code.cloudfoundry.org/lager"
)

type Statser struct {
	imageCloner    ImageCloner
	metricsEmitter MetricsEmitter
}

func IamStatser(imageCloner ImageCloner, metricsEmitter MetricsEmitter) *Statser {
	return &Statser{
		imageCloner:    imageCloner,
		metricsEmitter: metricsEmitter,
	}
}

func (m *Statser) Stats(logger lager.Logger, id string) (VolumeStats, error) {
	startTime := time.Now()
	defer func() {
		m.metricsEmitter.TryEmitDuration(logger, MetricImageStatsTime, time.Since(startTime))
	}()

	logger = logger.Session("groot-stats", lager.Data{"imageID": id})
	logger.Info("start")
	defer logger.Info("end")

	stats, err := m.imageCloner.Stats(logger, id)
	if err != nil {
		logger.Error("fetching-stats", err, lager.Data{"id": id})
		return VolumeStats{}, err
	}

	return stats, nil
}

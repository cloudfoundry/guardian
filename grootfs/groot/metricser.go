package groot

import (
	"time"

	"code.cloudfoundry.org/lager"
)

type Metricser struct {
	imageCloner    ImageCloner
	metricsEmitter MetricsEmitter
}

func IamMetricser(imageCloner ImageCloner, metricsEmitter MetricsEmitter) *Metricser {
	return &Metricser{
		imageCloner:    imageCloner,
		metricsEmitter: metricsEmitter,
	}
}

func (m *Metricser) Metrics(logger lager.Logger, id string) (VolumeMetrics, error) {
	startTime := time.Now()
	defer m.metricsEmitter.TryEmitDuration(logger, MetricImageStatsTime, time.Since(startTime))

	logger = logger.Session("groot-metrics", lager.Data{"imageID": id})
	logger.Info("start")
	defer logger.Info("end")

	metrics, err := m.imageCloner.Metrics(logger, id)
	if err != nil {
		logger.Error("fetching-metrics", err, lager.Data{"id": id})
		return VolumeMetrics{}, err
	}

	return metrics, nil
}

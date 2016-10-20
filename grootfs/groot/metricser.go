package groot

import (
	"fmt"

	"code.cloudfoundry.org/lager"
)

type Metricser struct {
	bundler Bundler
}

func IamMetricser(bundler Bundler) *Metricser {
	return &Metricser{
		bundler: bundler,
	}
}

func (m *Metricser) Metrics(logger lager.Logger, id string) (VolumeMetrics, error) {
	logger = logger.Session("groot-metrics", lager.Data{"bundleID": id})
	logger.Info("start")
	defer logger.Info("end")

	metrics, err := m.bundler.Metrics(logger, id)
	if err != nil {
		return VolumeMetrics{}, fmt.Errorf("fetching metrics for `%s`: %s", id, err)
	}

	return metrics, nil
}

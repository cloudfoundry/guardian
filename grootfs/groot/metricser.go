package groot

import "code.cloudfoundry.org/lager"

type Metricser struct {
	imageCloner ImageCloner
}

func IamMetricser(imageCloner ImageCloner) *Metricser {
	return &Metricser{
		imageCloner: imageCloner,
	}
}

func (m *Metricser) Metrics(logger lager.Logger, id string) (VolumeMetrics, error) {
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

package throttle

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager/v3"
)

//counterfeiter:generate . ContainerManager
type ContainerManager interface {
	Handles() ([]string, error)
	Metrics(log lager.Logger, handle string) (gardener.ActualContainerMetrics, error)
}

type ContainerMetricsSource struct {
	containerManager ContainerManager
}

func NewContainerMetricsSource(containerManager ContainerManager) MetricsSource {
	return &ContainerMetricsSource{
		containerManager: containerManager,
	}
}

func (c ContainerMetricsSource) CollectMetrics(logger lager.Logger) (map[string]gardener.ActualContainerMetrics, error) {
	logger = logger.Session("collect-metrics")

	logger.Info("start")
	defer logger.Info("finished")

	handles, err := c.containerManager.Handles()
	if err != nil {
		return nil, err
	}
	metrics := map[string]gardener.ActualContainerMetrics{}
	for _, h := range handles {
		metric, err := c.containerManager.Metrics(logger, h)
		if err != nil {
			logger.Error("failed-to-get-metrics", err, lager.Data{"handle": h})
			continue
		}
		metrics[h] = metric

	}
	return metrics, nil
}

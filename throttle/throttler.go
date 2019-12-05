package throttle

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager"
	multierror "github.com/hashicorp/go-multierror"
)

//go:generate counterfeiter . Enforcer
type Enforcer interface {
	Punish(logger lager.Logger, handle string) error
	Release(logger lager.Logger, handle string) error
}

//go:generate counterfeiter . MetricsSource
type MetricsSource interface {
	CollectMetrics(logger lager.Logger) (map[string]gardener.ActualContainerMetrics, error)
}

type Throttler struct {
	metricsSource MetricsSource
	enforcer      Enforcer
}

func NewThrottler(metricsSource MetricsSource, enforcer Enforcer) Throttler {
	return Throttler{
		metricsSource: metricsSource,
		enforcer:      enforcer,
	}
}

func (t Throttler) Run(logger lager.Logger) error {
	logger = logger.Session("throttle")
	logger.Info("starting")
	defer logger.Info("finished")
	metrics, err := t.metricsSource.CollectMetrics(logger)
	if err != nil {
		return err
	}

	var enforceErrs *multierror.Error
	for handle, metric := range metrics {
		err := t.throttle(logger, handle, metric)
		enforceErrs = multierror.Append(enforceErrs, err)
	}
	return enforceErrs.ErrorOrNil()
}

func (t Throttler) throttle(logger lager.Logger, handle string, metric gardener.ActualContainerMetrics) error {
	if metric.CPUEntitlement < metric.CPU.Usage {
		logger.Debug("punish-container", lager.Data{"handle": handle, "entitlement": metric.CPUEntitlement, "usage": metric.CPU.Usage})
		return t.enforcer.Punish(logger, handle)
	}

	logger.Debug("release-container", lager.Data{"handle": handle, "entitlement": metric.CPUEntitlement, "usage": metric.CPU.Usage})
	return t.enforcer.Release(logger, handle)
}

package metrics

import (
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metrics"
)

type Emitter struct{}

func NewEmitter(logger lager.Logger, metronEndpoint string) *Emitter {
	if metronEndpoint != "" {
		if err := dropsonde.Initialize(metronEndpoint, "grootfs"); err != nil {
			logger.Error("failed-to-initialize-metrics-emitter", err)
		}
	}
	return &Emitter{}
}

func (e *Emitter) TryEmitUsage(logger lager.Logger, name string, usage int64, units string) {
	if err := metrics.SendValue(name, float64(usage), units); err != nil {
		logger.Error("failed-to-emit-metric", err, lager.Data{
			"key":   name,
			"usage": usage,
		})
	}
}

func (e *Emitter) TryEmitDurationFrom(logger lager.Logger, name string, from time.Time) {
	duration := time.Since(from)

	if err := metrics.SendValue(name, float64(duration), "nanos"); err != nil {
		logger.Error("failed-to-emit-metric", err, lager.Data{
			"key":      name,
			"duration": duration,
		})
	}
}

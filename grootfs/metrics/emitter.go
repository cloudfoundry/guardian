package metrics

import (
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/dropsonde/metrics"
)

type Emitter struct {
}

func NewEmitter() *Emitter {
	return &Emitter{}
}

func (e *Emitter) TryEmitDuration(logger lager.Logger, name string, duration time.Duration) {
	if err := metrics.SendValue(name, float64(duration), "nanos"); err != nil {
		logger.Error("failed-to-emit-metric", err, lager.Data{
			"key":      name,
			"duration": duration,
		})
	}
}

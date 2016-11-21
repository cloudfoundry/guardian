package metrics

import (
	"time"

	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/dropsonde/metrics"
)

const dropsondeOrigin = "grootfs"

type Emitter struct {
}

func NewEmitter(metronEndpoint string) (*Emitter, error) {
	if err := dropsonde.Initialize(metronEndpoint, dropsondeOrigin); err != nil {
		return nil, err
	}

	return &Emitter{}, nil
}

func (e *Emitter) EmitDuration(name string, duration time.Duration) error {
	return metrics.SendValue(name, float64(duration), "nanos")
}

package metrics

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/dropsonde/envelopes"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

const errorSource = "grootfs.%s"

type Emitter struct {
}

func NewEmitter() *Emitter {
	return &Emitter{}
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

func (e *Emitter) TryEmitError(logger lager.Logger, command string, err error, exitCode int32) {
	message := err.Error()
	source := fmt.Sprintf(errorSource, command)

	errorEvent := events.Error{
		Code:    &exitCode,
		Source:  &source,
		Message: &message,
	}

	envelope := events.Envelope{
		Origin:    &source,
		EventType: events.Envelope_Error.Enum(),
		Error:     &errorEvent,
	}

	if err := envelopes.SendEnvelope(&envelope); err != nil {
		logger.Error("failed-to-emit-error-event", err, lager.Data{
			"errorEvent": errorEvent,
		})
	}
}

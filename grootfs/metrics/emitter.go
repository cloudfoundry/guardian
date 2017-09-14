package metrics

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/cloudfoundry/dropsonde/envelopes"
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/sonde-go/events"
)

const errorSource = "grootfs-error.%s"

//go:generate counterfeiter . SystemReporter
type SystemReporter interface {
	Report(lager.Logger, time.Duration)
}

type Emitter struct {
	systemReporter SystemReporter
}

func NewEmitter(systemReporter SystemReporter) *Emitter {
	return &Emitter{
		systemReporter: systemReporter,
	}
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

	e.systemReporter.Report(logger, duration)
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

func (e *Emitter) TryIncrementRunCount(command string, err error) {
	_ = metrics.IncrementCounter(fmt.Sprintf("grootfs-%s.run", command))

	if err != nil {
		_ = metrics.IncrementCounter(fmt.Sprintf("grootfs-%s.run.fail", command))
	} else {
		_ = metrics.IncrementCounter(fmt.Sprintf("grootfs-%s.run.success", command))
	}
}

package metrics

import (
	"time"

	"code.cloudfoundry.org/lager"
	dropsonde_metrics "github.com/cloudfoundry/dropsonde/metrics"
	"github.com/pivotal-golang/clock"
)

func sendMetric(key string, value int) {
	dropsonde_metrics.SendValue(key, float64(value), "Metric")
}

func sendDuration(duration time.Duration) {
	dropsonde_metrics.SendValue("MetricsReporting", float64(duration), "nanos")
}

type PeriodicMetronNotifier struct {
	Interval time.Duration
	Logger   lager.Logger
	Clock    clock.Clock

	metrics Metrics
	stopped chan struct{}
}

func NewPeriodicMetronNotifier(
	logger lager.Logger,
	metrics Metrics,
	interval time.Duration,
	clock clock.Clock,
) *PeriodicMetronNotifier {
	return &PeriodicMetronNotifier{
		Interval: interval,
		Logger:   logger,
		Clock:    clock,
		metrics:  metrics,

		stopped: make(chan struct{}),
	}
}

func (notifier PeriodicMetronNotifier) Start() {
	logger := notifier.Logger.Session("metrics-notifier", lager.Data{"interval": notifier.Interval.String()})
	logger.Info("starting")
	ticker := notifier.Clock.NewTicker(notifier.Interval)

	go func() {
		defer ticker.Stop()

		logger.Info("started", lager.Data{"time": notifier.Clock.Now()})
		defer logger.Info("finished")

		for {
			select {
			case <-ticker.C():
				startedAt := notifier.Clock.Now()

				for key, metric := range notifier.metrics {
					sendMetric(key, metric())
				}

				finishedAt := notifier.Clock.Now()
				sendDuration(finishedAt.Sub(startedAt))
			case <-notifier.stopped:
				return
			}
		}
	}()
}

func (notifier PeriodicMetronNotifier) Stop() {
	close(notifier.stopped)
}

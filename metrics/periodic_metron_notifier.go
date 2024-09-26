package metrics

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager/v3"
	dropsonde_metrics "github.com/cloudfoundry/dropsonde/metrics"
)

func sendMetric(key string, value int) error {
	return dropsonde_metrics.SendValue(key, float64(value), "Metric")
}

func sendDuration(duration time.Duration) error {
	return dropsonde_metrics.SendValue("MetricsReporting", float64(duration), "nanos")
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
					err := sendMetric(key, metric())
					if err != nil {
						logger.Debug("failed-to-send-metric", lager.Data{"error": err, "metric": key})
					}
				}

				finishedAt := notifier.Clock.Now()
				err := sendDuration(finishedAt.Sub(startedAt))
				if err != nil {
					logger.Debug("failed-to-send-metric", lager.Data{"error": err, "metric": "metrics-reporting-duration"})
				}
			case <-notifier.stopped:
				return
			}
		}
	}()
}

func (notifier PeriodicMetronNotifier) Stop() {
	close(notifier.stopped)
}

package metrics

import (
	"time"

	"code.cloudfoundry.org/lager"
	dropsonde_metrics "github.com/cloudfoundry/dropsonde/metrics"
	"github.com/pivotal-golang/clock"
)

type Metric string

func (name Metric) Send(value int) {
	dropsonde_metrics.SendValue(string(name), float64(value), "Metric")
}

type Duration string

func (name Duration) Send(duration time.Duration) {
	dropsonde_metrics.SendValue(string(name), float64(duration), "nanos")
}

const (
	loopDevices   = Metric("LoopDevices")
	backingStores = Metric("BackingStores")
	depotDirs     = Metric("DepotDirs")

	metricsReportingDuration = Duration("MetricsReporting")
)

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

				loopDevices.Send(notifier.metrics.LoopDevices())
				backingStores.Send(notifier.metrics.BackingStores())
				depotDirs.Send(notifier.metrics.DepotDirs())

				finishedAt := notifier.Clock.Now()
				metricsReportingDuration.Send(finishedAt.Sub(startedAt))
			case <-notifier.stopped:
				return
			}
		}
	}()
}

func (notifier PeriodicMetronNotifier) Stop() {
	close(notifier.stopped)
}

package groot

import (
	"time"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . Cleaner
type Cleaner interface {
	Clean(logger lager.Logger, cacheSize int64) (bool, error)
}

type cleaner struct {
	storeMeasurer    StoreMeasurer
	garbageCollector GarbageCollector
	locksmith        Locksmith
	metricsEmitter   MetricsEmitter
}

func IamCleaner(locksmith Locksmith, sm StoreMeasurer,
	gc GarbageCollector, metricsEmitter MetricsEmitter,
) *cleaner {
	return &cleaner{
		locksmith:        locksmith,
		storeMeasurer:    sm,
		garbageCollector: gc,
		metricsEmitter:   metricsEmitter,
	}
}

func (c *cleaner) Clean(logger lager.Logger, threshold int64) (bool, error) {
	logger = logger.Session("groot-cleaning")
	logger.Info("starting")

	defer c.metricsEmitter.TryEmitDurationFrom(logger, MetricImageCleanTime, time.Now())
	defer logger.Info("ending")

	if threshold > 0 {
		storeUsage, err := c.storeMeasurer.Usage(logger)
		if err != nil {
			return true, errorspkg.Wrap(err, "measuring store usage")
		}
		if storeUsage < threshold {
			return true, nil
		}
	} else if threshold < 0 {
		return true, errorspkg.New("Threshold must be greater than 0")
	}

	lockFile, err := c.locksmith.Lock(GlobalLockKey)
	if err != nil {
		return false, errorspkg.Wrap(err, "garbage collector acquiring lock")
	}

	unusedVolumes, err := c.garbageCollector.UnusedVolumes(logger)
	if err != nil {
		logger.Error("finding-unused-failed", err)
	}

	if err := c.garbageCollector.MarkUnused(logger, unusedVolumes); err != nil {
		logger.Error("marking-unused-failed", err)
	}

	if err := c.locksmith.Unlock(lockFile); err != nil {
		logger.Error("unlocking-failed", err)
	}

	return false, c.garbageCollector.Collect(logger)
}

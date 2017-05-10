package groot

import (
	"time"

	"code.cloudfoundry.org/lager"
	errorspkg "github.com/pkg/errors"
)

//go:generate counterfeiter . Cleaner
type Cleaner interface {
	Clean(logger lager.Logger, threshold int64, keepImages []string) (bool, error)
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

func (c *cleaner) Clean(logger lager.Logger, threshold int64, keepImages []string) (noop bool, err error) {
	defer c.metricsEmitter.TryEmitDurationFrom(logger, MetricImageCleanTime, time.Now())

	logger = logger.Session("groot-cleaning")
	logger.Info("starting")
	defer logger.Info("ending")

	if threshold > 0 {
		storeSize, err := c.storeMeasurer.MeasureStore(logger)
		if err != nil {
			return true, err
		}

		if threshold >= storeSize {
			return true, nil
		}
	} else if threshold < 0 {
		return true, errorspkg.New("Threshold must be greater than 0")
	}

	lockFile, err := c.locksmith.Lock(GlobalLockKey)
	if err != nil {
		return false, errorspkg.Wrap(err, "garbage collector acquiring lock")
	}

	if err := c.garbageCollector.MarkUnused(logger, keepImages); err != nil {
		logger.Error("marking-unused-failed", err)
	}

	if err := c.locksmith.Unlock(lockFile); err != nil {
		logger.Error("unlocking-failed", err)
	}

	return false, c.garbageCollector.Collect(logger)
}

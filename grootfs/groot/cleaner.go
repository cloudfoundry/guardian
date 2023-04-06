package groot

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/lager/v3"
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
		committedQuota, err := c.storeMeasurer.CommittedQuota(logger)
		if err != nil {
			return false, errorspkg.Wrap(err, "failed to calculate committed quota")
		}
		logger.Debug(fmt.Sprintf("commitedQuota in bytes is: %d", committedQuota))

		totalVolumesSize, err := c.storeMeasurer.TotalVolumesSize(logger)
		if err != nil {
			return false, errorspkg.Wrap(err, "failed to calculate total volumes size")
		}
		logger.Debug(fmt.Sprintf("totalVolumesSize in bytes is: %d", totalVolumesSize))
		logger.Debug(fmt.Sprintf("threshold in bytes is: %d", threshold))

		if (committedQuota + totalVolumesSize) < threshold {
			return true, nil
		}
	} else if threshold < 0 {
		return true, errorspkg.New("Threshold must be greater than 0")
	}

	return false, c.collectGarbage(logger)
}

func (c *cleaner) collectGarbage(logger lager.Logger) error {
	lockFile, err := c.locksmith.Lock(GlobalLockKey)
	if err != nil {
		return errorspkg.Wrap(err, "garbage collector acquiring lock")
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

	return c.garbageCollector.Collect(logger)
}

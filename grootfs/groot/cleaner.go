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
	logger = logger.Session("groot-cleaning")
	logger.Info("starting")
	defer c.metricsEmitter.TryEmitDurationFrom(logger, MetricImageCleanTime, time.Now())
	defer logger.Info("ending")

	storeSize, err := c.storeMeasurer.Size(logger)
	if err != nil {
		logger.Error("measuring-store-size-failed", err)
	}

	if storeSize != 0 {
		defer c.emitDiskCachePercentageMetric(logger, storeSize)
		defer c.emitDiskCommittedPercentageMetric(logger, storeSize)
	}

	if threshold > 0 {
		storeSize, err := c.storeMeasurer.Usage(logger)
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

	unusedVolumes, err := c.garbageCollector.UnusedVolumes(logger, keepImages)
	if err != nil {
		logger.Error("finding-unused-failed", err)
	}

	if storeSize != 0 {
		defer c.emitDiskPurgeableCachePercentageMetric(logger, unusedVolumes, storeSize)
	}

	if err := c.garbageCollector.MarkUnused(logger, unusedVolumes); err != nil {
		logger.Error("marking-unused-failed", err)
	}

	if err := c.locksmith.Unlock(lockFile); err != nil {
		logger.Error("unlocking-failed", err)
	}

	return false, c.garbageCollector.Collect(logger)
}

func (c *cleaner) emitDiskCachePercentageMetric(logger lager.Logger, storeSize int64) {
	cacheSize, err := c.storeMeasurer.Cache(logger)
	if err != nil {
		logger.Error("measuring-cache-size-failed", err)
		return
	}

	percentage := float64(cacheSize) / float64(storeSize) * 100.0
	c.metricsEmitter.TryEmitUsage(logger, MetricDiskCachePercentage, int64(percentage), "percentage")

	return
}

func (c *cleaner) emitDiskCommittedPercentageMetric(logger lager.Logger, storeSize int64) {
	committedSize, err := c.storeMeasurer.CommittedSize(logger)
	if err != nil {
		logger.Error("measuring-store-size-failed", err)
		return
	}

	percentage := float64(committedSize) / float64(storeSize) * 100.0
	c.metricsEmitter.TryEmitUsage(logger, MetricDiskCommittedPercentage, int64(percentage), "percentage")

	return
}

func (c *cleaner) emitDiskPurgeableCachePercentageMetric(logger lager.Logger, unusedVolumes []string, storeSize int64) {
	storeSize, err := c.storeMeasurer.Size(logger)
	if err != nil {
		return
	}

	purgeableSize, err := c.storeMeasurer.PurgeableCache(logger, unusedVolumes)
	if err != nil {
		return
	}

	if storeSize != 0 {
		purgeablePercentage := float64(purgeableSize) / float64(storeSize) * 100.0
		c.metricsEmitter.TryEmitUsage(logger, MetricDiskPurgeableCachePercentage, int64(purgeablePercentage), "percentage")
	}
}

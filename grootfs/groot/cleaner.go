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
	getLockTimeout   time.Duration
	cleaningTimeout  time.Duration
}

func IamCleaner(locksmith Locksmith, sm StoreMeasurer,
	gc GarbageCollector, metricsEmitter MetricsEmitter,
	getLockTimeout time.Duration, cleaningTimeout time.Duration) *cleaner {
	return &cleaner{
		locksmith:        locksmith,
		storeMeasurer:    sm,
		garbageCollector: gc,
		metricsEmitter:   metricsEmitter,
		getLockTimeout:   getLockTimeout,
		cleaningTimeout:  cleaningTimeout,
	}
}

type CleaningTimeoutError struct {
	Timeout time.Duration
}

func (e CleaningTimeoutError) Error() string {
	displayTimeout := fmt.Sprintf("%vs", e.Timeout.Seconds())
	return fmt.Sprintf("timed out cleaning after '%s'", displayTimeout)
}

func (c *cleaner) Clean(logger lager.Logger, threshold int64) (bool, error) {
	logger = logger.Session("groot-cleaning")
	cleaningTimeoutForLogs := fmt.Sprintf("%vs", c.cleaningTimeout.Seconds())
	logger.Info("starting", lager.Data{"cleaning_timeout": cleaningTimeoutForLogs})

	finishedCleaning := make(chan string, 1)
	var noop bool
	var err error

	go func() {
		noop, err = c.run(logger, threshold)
		finishedCleaning <- "all done cleaning!"
	}()

	select {
	case <-finishedCleaning:
		return noop, err
	case <-time.After(c.cleaningTimeout):
		return false, CleaningTimeoutError{Timeout: c.cleaningTimeout}
	}
}

func (c *cleaner) run(logger lager.Logger, threshold int64) (bool, error) {
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
	lockFile, err := c.locksmith.LockWithTimeout(GlobalLockKey, c.getLockTimeout)
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

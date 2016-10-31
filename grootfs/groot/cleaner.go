package groot

import "code.cloudfoundry.org/lager"

type Cleaner struct {
	storeMeasurer    StoreMeasurer
	garbageCollector GarbageCollector
	locksmith        Locksmith
}

func IamCleaner(locksmith Locksmith, sm StoreMeasurer, gc GarbageCollector) *Cleaner {
	return &Cleaner{
		locksmith:        locksmith,
		storeMeasurer:    sm,
		garbageCollector: gc,
	}
}

func (c *Cleaner) Clean(logger lager.Logger, threshold uint64) (bool, error) {
	logger = logger.Session("groot-cleaning")
	logger.Info("start")
	defer logger.Info("end")

	if threshold > 0 {
		storeSize, err := c.storeMeasurer.MeasureStore(logger)
		if err != nil {
			return true, err
		}

		if threshold >= storeSize {
			return true, nil
		}
	}

	lockFile, err := c.locksmith.Lock(GLOBAL_LOCK_KEY)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := c.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	return false, c.garbageCollector.Collect(logger)
}

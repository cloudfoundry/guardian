package groot

import "code.cloudfoundry.org/lager"

type Cleaner struct {
	garbageCollector GarbageCollector
	locksmith        Locksmith
}

func IamCleaner(locksmith Locksmith, gc GarbageCollector) *Cleaner {
	return &Cleaner{
		locksmith:        locksmith,
		garbageCollector: gc,
	}
}

func (c *Cleaner) Clean(logger lager.Logger) error {
	logger = logger.Session("groot-cleaning")
	logger.Info("start")
	defer logger.Info("end")

	lockFile, err := c.locksmith.Lock(GLOBAL_LOCK_KEY)
	if err != nil {
		return err
	}
	defer func() {
		if err := c.locksmith.Unlock(lockFile); err != nil {
			logger.Error("failed-to-unlock", err)
		}
	}()

	return c.garbageCollector.Collect(logger)
}

package runner

import (
	"os"

	"code.cloudfoundry.org/lager"
)

type UserCredential interface{}

func setUserCredential(runner *GardenRunner) {}

func (r *GardenRunner) setupDirsForUser() {}

func (r *RunningGarden) Cleanup() {
	r.logger.Info("cleanup-tempdirs")
	if err := os.RemoveAll(r.TmpDir); err != nil {
		r.logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.TmpDir})
	} else {
		r.logger.Info("tempdirs-removed")
	}
}

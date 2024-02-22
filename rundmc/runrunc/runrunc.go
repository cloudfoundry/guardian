package runrunc

import (
	"os/exec"
	"time"

	"code.cloudfoundry.org/guardian/rundmc/deleter"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager/v3"
)

// da doo
type RunRunc struct {
	*Creator
	*Execer
	*OomWatcher
	*Statser
	*Stater
	*deleter.Deleter
	*BundleManager
}

//counterfeiter:generate . RuncBinary
type RuncBinary interface {
	RunCommand(bundlePath, pidfilePath, logfilePath, id string, extraGlobalArgs []string) *exec.Cmd
	ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd
	EventsCommand(id string) *exec.Cmd
	StateCommand(id, logFile string) *exec.Cmd
	StatsCommand(id, logFile string) *exec.Cmd
	DeleteCommand(id string, force bool, logFile string) *exec.Cmd
}

//counterfeiter:generate . Depot
type Depot interface {
	Create(log lager.Logger, handle string, bundle goci.Bndl) (string, error)
	CreatedTime(log lager.Logger, handle string) (time.Time, error)
	Lookup(log lager.Logger, handle string) (path string, err error)
	Load(log lager.Logger, handle string) (bundle goci.Bndl, err error)
	Handles() ([]string, error)
	Destroy(log lager.Logger, handle string) error
}

func New(
	creator *Creator,
	execer *Execer,
	oomWatcher *OomWatcher,
	statser *Statser,
	stater *Stater,
	deleter *deleter.Deleter,
	bundleManager *BundleManager,
) *RunRunc {

	return &RunRunc{
		Creator:       creator,
		Execer:        execer,
		OomWatcher:    oomWatcher,
		Statser:       statser,
		Stater:        stater,
		Deleter:       deleter,
		BundleManager: bundleManager,
	}
}

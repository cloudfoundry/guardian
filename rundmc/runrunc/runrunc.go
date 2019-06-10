package runrunc

import (
	"os/exec"
	"time"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

// da doo
type RunRunc struct {
	*Execer
	*Creator
	*OomWatcher
	*Statser
	*Stater
	*Deleter
	*Infoer
}

//go:generate counterfeiter . RuncBinary
type RuncBinary interface {
	RunCommand(bundlePath, pidfilePath, logfilePath, id string, extraGlobalArgs []string) *exec.Cmd
	ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd
	EventsCommand(id string) *exec.Cmd
	StateCommand(id, logFile string) *exec.Cmd
	StatsCommand(id, logFile string) *exec.Cmd
	DeleteCommand(id string, force bool, logFile string) *exec.Cmd
}

//go:generate counterfeiter . Depot
type Depot interface {
	CreatedTime(log lager.Logger, handle string) (time.Time, error)
	Lookup(log lager.Logger, handle string) (path string, err error)
	Load(log lager.Logger, handle string) (bundle goci.Bndl, err error)
}

func New(
	runner commandrunner.CommandRunner, runcCmdRunner RuncCmdRunner,
	runc RuncBinary, runcExtraArgs []string, execer *Execer, statser *Statser,
	infoer *Infoer,
) *RunRunc {
	stater := NewStater(runcCmdRunner, runc)
	oomWatcher := NewOomWatcher(runner, runc)

	return &RunRunc{
		Creator:    NewCreator(runc, runcExtraArgs, runner, oomWatcher),
		Execer:     execer,
		OomWatcher: oomWatcher,
		Statser:    statser,
		Stater:     stater,
		Deleter:    NewDeleter(runcCmdRunner, runc, stater),
		Infoer:     infoer,
	}
}

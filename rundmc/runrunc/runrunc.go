package runrunc

import (
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
)

// da doo
type RunRunc struct {
	*Execer
	*Creator
	*OomWatcher
	*Statser
	*Stater
	*Killer
	*Deleter
}

//go:generate counterfeiter . RuncBinary
type RuncBinary interface {
	RunCommand(bundlePath, pidfilePath, logfilePath, id string, extraGlobalArgs []string) *exec.Cmd
	ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd
	EventsCommand(id string) *exec.Cmd
	StateCommand(id, logFile string) *exec.Cmd
	StatsCommand(id, logFile string) *exec.Cmd
	KillCommand(id, signal, logFile string) *exec.Cmd
	DeleteCommand(id string, force bool, logFile string) *exec.Cmd
}

func New(
	runner commandrunner.CommandRunner, runcCmdRunner RuncCmdRunner,
	runc RuncBinary, runcExtraArgs []string, execer *Execer, statser *Statser,
	bundleLookuper func(string) (string, error),
) *RunRunc {
	stater := NewStater(runcCmdRunner, runc)
	oomWatcher := NewOomWatcher(runner, runc)

	return &RunRunc{
		Creator:    NewCreator(runc, runcExtraArgs, runner, oomWatcher, bundleLookuper),
		Execer:     execer,
		OomWatcher: oomWatcher,
		Statser:    statser,
		Stater:     stater,
		Killer:     NewKiller(runcCmdRunner, runc),
		Deleter:    NewDeleter(runcCmdRunner, runc, stater),
	}
}

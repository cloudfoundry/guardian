package runrunc

import (
	"os/exec"

	"code.cloudfoundry.org/commandrunner"
)

const DefaultRootPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
const DefaultPath = "PATH=/usr/local/bin:/usr/bin:/bin"

// da doo
type RunRunc struct {
	commandRunner commandrunner.CommandRunner
	runc          RuncBinary

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
	ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd
	EventsCommand(id string) *exec.Cmd
	StateCommand(id, logFile string) *exec.Cmd
	StatsCommand(id, logFile string) *exec.Cmd
	KillCommand(id, signal, logFile string) *exec.Cmd
	DeleteCommand(id string, force bool, logFile string) *exec.Cmd
}

func New(
	runner commandrunner.CommandRunner, runcCmdRunner RuncCmdRunner,
	runc RuncBinary, runcPath string, runcExtraArgs []string, execer *Execer, statser *Statser,
) *RunRunc {
	return &RunRunc{
		Creator:    NewCreator(runcPath, runcExtraArgs, runner),
		Execer:     execer,
		OomWatcher: NewOomWatcher(runner, runc),
		Statser:    statser,
		Stater:     NewStater(runcCmdRunner, runc),
		Killer:     NewKiller(runcCmdRunner, runc),
		Deleter:    NewDeleter(runcCmdRunner, runc),
	}
}

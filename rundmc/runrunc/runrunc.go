package runrunc

import (
	"os/exec"

	"github.com/cloudfoundry/gunk/command_runner"
)

const DefaultRootPath = "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
const DefaultPath = "PATH=/usr/local/bin:/usr/bin:/bin"

// da doo
type RunRunc struct {
	commandRunner command_runner.CommandRunner
	runc          RuncBinary

	*Execer
	*Starter
	*OomWatcher
	*Statser
	*Stater
	*Killer
}

//go:generate counterfeiter . RuncBinary
type RuncBinary interface {
	ExecCommand(id, processJSONPath, pidFilePath string) *exec.Cmd
	EventsCommand(id string) *exec.Cmd
	StateCommand(id string) *exec.Cmd
	StatsCommand(id string) *exec.Cmd
	KillCommand(id, signal string) *exec.Cmd
	DeleteCommand(id string) *exec.Cmd
}

func New(tracker ProcessTracker, runner command_runner.CommandRunner, pidgen UidGenerator, runc RuncBinary, dadooPath string, execPreparer *ExecPreparer) *RunRunc {
	return &RunRunc{
		Starter: NewStarter(dadooPath, runner),
		Execer:  NewExecer(runc, pidgen, tracker, execPreparer),

		OomWatcher: NewOomWatcher(runner, runc),
		Statser:    NewStatser(runner, runc),
		Stater:     NewStater(runner, runc),
		Killer:     NewKiller(runner, runc),
	}
}

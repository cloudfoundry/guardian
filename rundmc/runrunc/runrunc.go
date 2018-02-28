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
	runc RuncBinary, dadooPath, runcPath string, runcExtraArgs []string, bundleLoader BundleLoader, processBuilder ProcessBuilder,
	mkdirer Mkdirer, userLookuper UserLookupper, execRunner ExecRunner, uidGenerator UidGenerator,
) *RunRunc {
	return &RunRunc{
		Creator: NewCreator(runcPath, runcExtraArgs, runner),
		Execer:  NewExecer(bundleLoader, processBuilder, mkdirer, userLookuper, execRunner, uidGenerator),

		OomWatcher: NewOomWatcher(runner, runc),
		Statser:    NewStatser(runcCmdRunner, runc),
		Stater:     NewStater(runcCmdRunner, runc),
		Killer:     NewKiller(runcCmdRunner, runc),
		Deleter:    NewDeleter(runcCmdRunner, runc),
	}
}

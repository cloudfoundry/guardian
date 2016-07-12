package runrunc

import (
	"os/exec"

	"code.cloudfoundry.org/lager"
)

type Deleter struct {
	runner RuncCmdRunner
	runc   RuncBinary
}

func NewDeleter(runner RuncCmdRunner, runc RuncBinary) *Deleter {
	return &Deleter{
		runner: runner,
		runc:   runc,
	}
}

func (d *Deleter) Delete(log lager.Logger, handle string) error {
	log = log.Session("delete", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	return d.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		return d.runc.DeleteCommand(handle, logFile)
	})
}

package runrunc

import (
	"os/exec"

	"code.cloudfoundry.org/lager"
)

type RuncDeleter struct {
	runner RuncCmdRunner
	runc   RuncBinary
}

func NewDeleter(runner RuncCmdRunner, runc RuncBinary) *RuncDeleter {
	return &RuncDeleter{
		runner: runner,
		runc:   runc,
	}
}

func (d *RuncDeleter) Delete(log lager.Logger, handle string, force bool) error {
	log = log.Session("runc-delete", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	return d.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		return d.runc.DeleteCommand(handle, force, logFile)
	})
}

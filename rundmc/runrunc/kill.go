package runrunc

import (
	"os/exec"

	"code.cloudfoundry.org/lager"
)

type Killer struct {
	runner RuncCmdRunner
	runc   RuncBinary
}

func NewKiller(runner RuncCmdRunner, runc RuncBinary) *Killer {
	return &Killer{
		runner,
		runc,
	}
}

// Kill a bundle using 'runc kill'
func (r *Killer) Kill(log lager.Logger, handle string) error {
	log = log.Session("kill", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	return r.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		return r.runc.KillCommand(handle, "KILL", logFile)
	})
}

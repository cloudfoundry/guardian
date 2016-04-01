package runrunc

import (
	"os/exec"

	"github.com/pivotal-golang/lager"
)

type Killer struct {
	loggingRunner RuncCmdRunner
	runc          RuncBinary
}

func NewKiller(loggingRunner RuncCmdRunner, runc RuncBinary) *Killer {
	return &Killer{
		loggingRunner,
		runc,
	}
}

// Kill a bundle using 'runc kill'
func (r *Killer) Kill(log lager.Logger, handle string) error {
	log = log.Session("kill", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	return r.loggingRunner.RunAndLog(log, func(logFile string) *exec.Cmd {
		return r.runc.KillCommand(handle, "KILL", logFile)
	})
}

package runrunc

import (
	"os/exec"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . RuncStater
type RuncStater interface {
	State(lager.Logger, string) (State, error)
}

type Deleter struct {
	runner RuncCmdRunner
	runc   RuncBinary
	stater RuncStater
}

func NewDeleter(runner RuncCmdRunner, runc RuncBinary, stater RuncStater) *Deleter {
	return &Deleter{
		runner: runner,
		runc:   runc,
		stater: stater,
	}
}

func (d *Deleter) Delete(log lager.Logger, handle string) error {
	log = log.Session("delete", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	state, err := d.stater.State(log, handle)
	if err != nil {
		log.Info("state-failed-skipping-delete", lager.Data{"error": err.Error()})
		return nil
	}

	log.Info("state", lager.Data{
		"state": state,
	})
	if shouldDelete(state.Status) {
		return d.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
			return d.runc.DeleteCommand(handle, state.Status == RunningStatus, logFile)
		})
	}

	return nil
}

func shouldDelete(status Status) bool {
	return status == CreatedStatus || status == StoppedStatus || status == RunningStatus
}

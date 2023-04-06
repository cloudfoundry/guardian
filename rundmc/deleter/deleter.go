package deleter

import (
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/lager/v3"
)

//go:generate counterfeiter . RuntimeStater

type RuntimeStater interface {
	State(log lager.Logger, handle string) (rundmc.State, error)
}

//go:generate counterfeiter . RuntimeDeleter

type RuntimeDeleter interface {
	Delete(log lager.Logger, handle string, force bool) error
}

type Deleter struct {
	stater  RuntimeStater
	deleter RuntimeDeleter
}

func NewDeleter(stater RuntimeStater, deleter RuntimeDeleter) *Deleter {
	return &Deleter{
		stater:  stater,
		deleter: deleter,
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
		return d.deleter.Delete(log, handle, state.Status == rundmc.RunningStatus)
	}

	return nil
}

func shouldDelete(status rundmc.Status) bool {
	return status == rundmc.CreatedStatus || status == rundmc.StoppedStatus || status == rundmc.RunningStatus
}

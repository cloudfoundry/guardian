package runcontainerd

import "code.cloudfoundry.org/lager/v3"

//go:generate counterfeiter . Runtime
type Runtime interface {
	Delete(log lager.Logger, id string) error
	RemoveBundle(log lager.Logger, id string) error
}

type NerdDeleter struct {
	runtime Runtime
}

func NewDeleter(runtime Runtime) *NerdDeleter {
	return &NerdDeleter{
		runtime: runtime,
	}
}

func (d *NerdDeleter) Delete(log lager.Logger, handle string, _ bool) error {
	log = log.Session("nerd-delete", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	if err := d.runtime.Delete(log, handle); err != nil {
		return err
	}

	return d.runtime.RemoveBundle(log, handle)
}

package throttle

import (
	"code.cloudfoundry.org/lager/v3"
	multierror "github.com/hashicorp/go-multierror"
)

type CompositeRunnable struct {
	runnables []Runnable
}

func NewCompositeRunnable(runnables ...Runnable) CompositeRunnable {
	return CompositeRunnable{
		runnables: runnables,
	}
}

func (r CompositeRunnable) Run(logger lager.Logger) error {
	var merr *multierror.Error

	for _, r := range r.runnables {
		err := r.Run(logger)
		merr = multierror.Append(merr, err)
	}

	return merr.ErrorOrNil()
}

package peas

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/peas/processwaiter"
	"code.cloudfoundry.org/lager/v3"
	multierror "github.com/hashicorp/go-multierror"
)

type PeaCleaner struct {
	Deleter      Deleter
	Volumizer    Volumizer
	Waiter       processwaiter.ProcessWaiter
	Runtime      Runtime
	PeaPidGetter PeaPidGetter
}

//go:generate counterfeiter . Runtime
type Runtime interface {
	ContainerHandles() ([]string, error)
	ContainerPeaHandles(log lager.Logger, id string) ([]string, error)
}

//go:generate counterfeiter . PeaPidGetter
type PeaPidGetter interface {
	GetPeaPid(logger lager.Logger, _, peaID string) (int, error)
}

//go:generate counterfeiter . Deleter
type Deleter interface {
	Delete(log lager.Logger, handle string) error
}

func NewPeaCleaner(deleter Deleter, volumizer Volumizer, runtime Runtime, peaPidGetter PeaPidGetter) gardener.PeaCleaner {
	return &PeaCleaner{
		Deleter:      deleter,
		Volumizer:    volumizer,
		Waiter:       processwaiter.WaitOnProcess,
		Runtime:      runtime,
		PeaPidGetter: peaPidGetter,
	}
}

func (p *PeaCleaner) Clean(log lager.Logger, handle string) error {
	log = log.Session("clean-pea", lager.Data{"peaHandle": handle})
	log.Info("start")
	defer log.Info("end")

	var result *multierror.Error
	err := p.Deleter.Delete(log, handle)
	if err != nil {
		result = multierror.Append(result, err)
	}
	err = p.Volumizer.Destroy(log, handle)
	if err != nil {
		result = multierror.Append(result, err)
	}

	return result.ErrorOrNil()
}

func (p *PeaCleaner) CleanAll(log lager.Logger) error {
	log = log.Session("clean-all-peas")
	log.Info("start")
	defer log.Info("end")

	sandboxHandles, err := p.Runtime.ContainerHandles()
	if err != nil {
		return err
	}

	for _, sandboxHandle := range sandboxHandles {
		peaHandles, err := p.Runtime.ContainerPeaHandles(log, sandboxHandle)
		if err != nil {
			log.Error("error-getting-peas", err, lager.Data{"sandboxHandle": sandboxHandle})
			continue
		}

		for _, peaHandle := range peaHandles {
			peaPID, err := p.PeaPidGetter.GetPeaPid(log, sandboxHandle, peaHandle)
			if err != nil {
				log.Error("error-getting-pea-pid", err, lager.Data{"sandboxHandle": sandboxHandle, "peaHandle": peaHandle})
				continue
			}

			go func(peaHandle string, peaPID int) {
				log.Info("pea-cleaner-goroutine-started", lager.Data{"peaHandle": peaHandle, "peaPID": peaPID})
				defer log.Info("pea-cleaner-goroutine-ended", lager.Data{"peaHandle": peaHandle, "peaPID": peaPID})
				if err := p.Waiter.Wait(peaPID); err != nil {
					log.Error("error-waiting-on-pea", err, lager.Data{"peaHandle": peaHandle, "peaPID": peaPID})
					return
				}

				if err := p.Clean(log, peaHandle); err != nil {
					log.Error("error-cleaning-up-pea", err, lager.Data{"peaHandle": peaHandle, "peaPID": peaPID})
					return
				}
			}(peaHandle, peaPID)

		}
	}

	return nil
}

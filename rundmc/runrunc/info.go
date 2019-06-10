package runrunc

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

type Infoer struct {
	depot Depot
}

func NewInfoer(depot Depot) *Infoer {
	return &Infoer{
		depot: depot,
	}
}

func (i *Infoer) BundleInfo(log lager.Logger, handle string) (string, goci.Bndl, error) {
	bundlePath, err := i.depot.Lookup(log, handle)
	if err == depot.ErrDoesNotExist {
		return "", goci.Bndl{}, garden.ContainerNotFoundError{Handle: handle}
	}
	if err != nil {
		return "", goci.Bndl{}, err
	}

	bundle, err := i.depot.Load(log, handle)

	return bundlePath, bundle, err
}

func (i *Infoer) Handles() ([]string, error) {
	return i.depot.Handles()
}

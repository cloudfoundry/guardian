package runrunc

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

type BundleManager struct {
	depot Depot
}

func NewBundleManager(depot Depot) *BundleManager {
	return &BundleManager{
		depot: depot,
	}
}

func (i *BundleManager) BundleInfo(log lager.Logger, handle string) (string, goci.Bndl, error) {
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

func (i *BundleManager) BundleIDs() ([]string, error) {
	return i.depot.Handles()
}

func (i *BundleManager) RemoveBundle(log lager.Logger, handle string) error {
	return i.depot.Destroy(log, handle)
}

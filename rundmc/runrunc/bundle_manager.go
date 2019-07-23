package runrunc

import (
	"os"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ProcessDepot
type ProcessDepot interface {
	ListProcessDirs(log lager.Logger, sandboxHandle string) ([]string, error)
}

type BundleManager struct {
	depot        Depot
	processDepot ProcessDepot
}

func NewBundleManager(depot Depot, processDepot ProcessDepot) *BundleManager {
	return &BundleManager{
		depot:        depot,
		processDepot: processDepot,
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

func (i *BundleManager) ContainerHandles() ([]string, error) {
	return i.depot.Handles()
}

func (i *BundleManager) ContainerPeaHandles(log lager.Logger, sandboxHandle string) ([]string, error) {
	processDirs, err := i.processDepot.ListProcessDirs(log, sandboxHandle)
	if err != nil {
		return nil, err
	}

	peaDirs, err := filterStringSlice(processDirs, isPeaDir(log))
	if err != nil {
		return nil, err
	}

	peas := []string{}
	for _, dir := range peaDirs {
		peas = append(peas, filepath.Base(dir))
	}

	return peas, nil
}

func isPeaDir(log lager.Logger) func(string) (bool, error) {
	return func(path string) (bool, error) {
		if configJSONExists, err := fileExists(filepath.Join(path, "config.json")); !configJSONExists {
			return false, err
		}

		if pidfileExists, err := fileExists(filepath.Join(path, "pidfile")); !pidfileExists {
			log.Error("error-pea-pidfile-does-not-exist", err, lager.Data{"path": path})
			return false, err
		}
		return true, nil
	}
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func filterStringSlice(slice []string, filter func(string) (bool, error)) ([]string, error) {
	var filtered []string

	for _, item := range slice {
		include, err := filter(item)
		if err != nil {
			return nil, err
		}
		if include {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (i *BundleManager) RemoveBundle(log lager.Logger, handle string) error {
	return i.depot.Destroy(log, handle)
}

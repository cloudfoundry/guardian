package execrunner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ProcessDepot
type ProcessDepot interface {
	CreateProcessDir(log lager.Logger, sandboxHandle, processID string) (string, error)
}

type ProcessDirDepot struct {
	bundleLookupper depot.BundleLookupper
}

func NewProcessDirDepot(bundleLookupper depot.BundleLookupper) ProcessDirDepot {
	return ProcessDirDepot{bundleLookupper: bundleLookupper}
}

func (d ProcessDirDepot) CreateProcessDir(log lager.Logger, sandboxHandle, processID string) (string, error) {
	bundlePath, err := d.bundleLookupper.Lookup(log, sandboxHandle)
	if err != nil {
		return "", err
	}

	processPath := filepath.Join(bundlePath, "processes", processID)
	if _, err := os.Stat(processPath); err == nil {
		return "", errors.New(fmt.Sprintf("process ID '%s' already in use", processID))
	}

	if err := os.MkdirAll(processPath, 0700); err != nil {
		return "", err
	}

	return processPath, nil
}

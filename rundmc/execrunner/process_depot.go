package execrunner

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . ProcessDepot
type ProcessDepot interface {
	CreateProcessDir(log lager.Logger, sandboxHandle, processID string) (string, error)
	LookupProcessDir(log lager.Logger, sandboxHandle, processID string) (string, error)
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

func (d ProcessDirDepot) LookupProcessDir(log lager.Logger, sandboxHandle, processID string) (string, error) {
	bundlePath, err := d.bundleLookupper.Lookup(log, sandboxHandle)
	if err != nil {
		return "", err
	}

	processPath := filepath.Join(bundlePath, "processes", processID)
	if _, err := os.Stat(processPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("process %s not found", processID)
		}
		return "", err
	}

	return processPath, nil
}

func (d ProcessDirDepot) ListProcessDirs(log lager.Logger, sandboxHandle string) ([]string, error) {
	bundlePath, err := d.bundleLookupper.Lookup(log, sandboxHandle)
	if err != nil {
		return []string{}, err
	}

	processesDirContents, err := ioutil.ReadDir(filepath.Join(bundlePath, "processes"))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}

		return []string{}, err
	}

	var processDirs []string
	for _, fileInfo := range processesDirContents {
		if fileInfo.IsDir() {
			processDirs = append(processDirs, filepath.Join(bundlePath, "processes", fileInfo.Name()))
		}
	}

	return processDirs, nil
}

package execrunner

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

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

	if err := os.MkdirAll(processPath, 0755); err != nil {
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

func (d ProcessDirDepot) CreatedTime(log lager.Logger, processID string) (time.Time, error) {
	processPath, err := d.findProcessPath(log, processID)
	if err != nil {
		return time.Time{}, err
	}

	info, err := os.Stat(filepath.Join(processPath, "pidfile"))
	if err != nil {
		return time.Time{}, fmt.Errorf("process pidfile does not exist: %#v", err)
	}

	return info.ModTime(), nil
}

func (d ProcessDirDepot) findProcessPath(log lager.Logger, processID string) (string, error) {
	sandboxHandles, _ := d.bundleLookupper.Handles()
	for _, sandboxHandle := range sandboxHandles {
		processPath, err := d.LookupProcessDir(log, sandboxHandle, processID)
		if err == nil {
			return processPath, nil
		}
	}

	return "", fmt.Errorf("process %s not found", processID)
}

package metrics

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"runtime"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Metrics

type Metrics interface {
	NumCPU() int
	NumGoroutine() int
	LoopDevices() int
	BackingStores() int
	DepotDirs() int
}

type metrics struct {
	backingStoresPath string
	depotPath         string
	logger            lager.Logger
}

func NewMetrics(logger lager.Logger, backingStoresPath, depotPath string) Metrics {
	return &metrics{
		backingStoresPath: backingStoresPath,
		depotPath:         depotPath,
		logger:            logger.Session("metrics"),
	}
}

func (m *metrics) NumCPU() int {
	return runtime.NumCPU()
}

func (m *metrics) NumGoroutine() int {
	return runtime.NumGoroutine()
}

func (m *metrics) LoopDevices() int {
	devices, err := exec.Command("losetup", "-a").CombinedOutput()
	if err != nil {
		m.logger.Error("cannot-get-loop-devices", fmt.Errorf("%s, out: %s", err, string(devices)))
		return -1
	}
	return bytes.Count(devices, []byte("\n"))
}

func (m *metrics) BackingStores() int {
	if m.backingStoresPath == "" {
		// graph is disabled
		return -1
	}

	entries, err := ioutil.ReadDir(m.backingStoresPath)
	if err != nil {
		m.logger.Error("cannot-get-backing-stores", err)
		return -1
	}

	return len(entries)
}

func (m *metrics) DepotDirs() int {
	entries, err := ioutil.ReadDir(m.depotPath)
	if err != nil {
		m.logger.Error("cannot-get-depot-dirs", err)
		return -1
	}

	return len(entries)
}

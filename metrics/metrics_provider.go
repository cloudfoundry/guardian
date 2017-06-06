package metrics

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"runtime"

	"code.cloudfoundry.org/lager"
)

type MetricsProvider struct {
	backingStoresPath string
	depotPath         string
	logger            lager.Logger
}

func NewMetricsProvider(logger lager.Logger, backingStoresPath, depotPath string) *MetricsProvider {
	return &MetricsProvider{
		backingStoresPath: backingStoresPath,
		depotPath:         depotPath,
		logger:            logger.Session("metrics"),
	}
}

func (m *MetricsProvider) NumCPU() int {
	return runtime.NumCPU()
}

func (m *MetricsProvider) NumGoroutine() int {
	return runtime.NumGoroutine()
}

func (m *MetricsProvider) LoopDevices() int {
	devices, err := exec.Command("losetup", "-a").CombinedOutput()
	if err != nil {
		m.logger.Error("cannot-get-loop-devices", fmt.Errorf("%s, out: %s", err, string(devices)))
		return -1
	}
	return bytes.Count(devices, []byte("\n"))
}

func (m *MetricsProvider) BackingStores() int {
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

func (m *MetricsProvider) DepotDirs() int {
	entries, err := ioutil.ReadDir(m.depotPath)
	if err != nil {
		m.logger.Error("cannot-get-depot-dirs", err)
		return -1
	}

	return len(entries)
}

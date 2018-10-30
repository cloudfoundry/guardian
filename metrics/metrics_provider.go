package metrics

import (
	"io/ioutil"
	"runtime"

	"code.cloudfoundry.org/lager"
)

type MetricsProvider struct {
	depotPath string
	logger    lager.Logger
}

func NewMetricsProvider(logger lager.Logger, depotPath string) *MetricsProvider {
	return &MetricsProvider{
		depotPath: depotPath,
		logger:    logger.Session("metrics"),
	}
}

func (m *MetricsProvider) NumCPU() int {
	return runtime.NumCPU()
}

func (m *MetricsProvider) NumGoroutine() int {
	return runtime.NumGoroutine()
}

// TODO: remove this eventually
func (m *MetricsProvider) LoopDevices() int {
	return 0
}

// TODO: remove this eventually
func (m *MetricsProvider) BackingStores() int {
	return 0
}

func (m *MetricsProvider) DepotDirs() int {
	entries, err := ioutil.ReadDir(m.depotPath)
	if err != nil {
		m.logger.Error("cannot-get-depot-dirs", err)
		return -1
	}

	return len(entries)
}

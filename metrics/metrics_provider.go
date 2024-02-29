package metrics

import (
	"os"
	"runtime"
	"sync"

	"code.cloudfoundry.org/lager/v3"
)

type MetricsProvider struct {
	lock                 sync.Mutex
	unkillableContainers map[string]struct{}
	depotPath            string
	logger               lager.Logger
}

func NewMetricsProvider(logger lager.Logger, depotPath string) *MetricsProvider {
	return &MetricsProvider{
		depotPath:            depotPath,
		logger:               logger.Session("metrics"),
		unkillableContainers: map[string]struct{}{},
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
	entries, err := os.ReadDir(m.depotPath)
	if err != nil {
		m.logger.Error("cannot-get-depot-dirs", err)
		return -1
	}

	return len(entries)
}

func (m *MetricsProvider) UnkillableContainers() int {
	return len(m.unkillableContainers)
}

func (m *MetricsProvider) RegisterUnkillableContainer(containerId string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.unkillableContainers[containerId] = struct{}{}
}

func (m *MetricsProvider) RegisterKillableContainer(containerId string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.unkillableContainers, containerId)
}

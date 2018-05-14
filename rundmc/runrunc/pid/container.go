package pid

import (
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Depot
type Depot interface {
	Lookup(lager.Logger, string) (string, error)
}

//go:generate counterfeiter . PidFileReader
type PidFileReader interface {
	Pid(pidFilePath string) (int, error)
}

type ContainerPidGetter struct {
	Depot         Depot
	PidFileReader PidFileReader
}

func (f *ContainerPidGetter) GetPid(logger lager.Logger, containerHandle string) (int, error) {
	bundlePath, err := f.Depot.Lookup(logger, containerHandle)
	if err != nil {
		return 0, err
	}

	return f.PidFileReader.Pid(filepath.Join(bundlePath, "pidfile"))
}

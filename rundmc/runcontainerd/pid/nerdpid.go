package nerdpid

import (
	"errors"

	"code.cloudfoundry.org/lager"
)

type PidGetter struct {
}

func NewPidGetter() *PidGetter {
	return &PidGetter{}
}

func (f *PidGetter) GetPid(logger lager.Logger, containerHandle string) (int, error) {
	return 0, errors.New("GetPid is not implemented")
	// bundlePath, err := f.Depot.Lookup(logger, containerHandle)
	// if err != nil {
	// 	return 0, err
	// }
	//
	// return f.PidFileReader.Pid(filepath.Join(bundlePath, "pidfile"))
}

func (f *PidGetter) GetPeaPid(logger lager.Logger, containerHandle, peaID string) (int, error) {
	return 0, errors.New("GetPeaPid is not implemented")
	// bundlePath, err := f.Depot.Lookup(logger, containerHandle)
	// if err != nil {
	// 	return 0, err
	// }
	//
	// return f.PidFileReader.Pid(filepath.Join(bundlePath, "processes", peaID, "pidfile"))
}

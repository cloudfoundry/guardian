package runcontainerd

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

//go:generate counterfeiter . CgroupManager
type CgroupManager interface {
	SetUseMemoryHierarchy(handle string) error
}

type cgroupManager struct {
	runcRoot  string
	namespace string
}

type containerState struct {
	CgroupPaths cgroupPaths `json:"cgroup_paths"`
}

type cgroupPaths struct {
	Memory string
}

func NewCgroupManager(runcRoot, namespace string) CgroupManager {
	return cgroupManager{
		runcRoot:  runcRoot,
		namespace: namespace,
	}
}

func (m cgroupManager) SetUseMemoryHierarchy(handle string) error {
	statePath := filepath.Join(m.runcRoot, m.namespace, handle, "state.json")
	stateFile, err := os.Open(statePath)
	if err != nil {
		return err
	}
	defer stateFile.Close()

	var state containerState
	err = json.NewDecoder(stateFile).Decode(&state)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(state.CgroupPaths.Memory, "memory.use_hierarchy"), []byte("1"), os.ModePerm)
}

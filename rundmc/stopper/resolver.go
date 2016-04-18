package stopper

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type state struct {
	CgroupPaths map[string]string `json:"cgroup_paths"`
}

type resolver struct {
	stateStore string
}

func NewRuncStateCgroupPathResolver(stateStorePath string) *resolver {
	return &resolver{
		stateStore: stateStorePath,
	}
}

func (r resolver) Resolve(name, subsystem string) (string, error) {
	stateJson, err := os.Open(filepath.Join(r.stateStore, name, "state.json"))
	if err != nil {
		return "", err
	}

	var s state
	if err := json.NewDecoder(stateJson).Decode(&s); err != nil {
		return "", err
	}

	return s.CgroupPaths["devices"], nil
}

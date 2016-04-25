package properties

import (
	"encoding/json"
	"os"
)

func Load(path string) (*Manager, error) {
	f, err := os.Open(path)
	if err != nil {
		return NewManager(), nil
	}

	var mgr Manager
	if err := json.NewDecoder(f).Decode(&mgr); err != nil {
		return nil, err
	}

	return &mgr, nil
}

func Save(path string, mgr *Manager) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	return json.NewEncoder(f).Encode(mgr)
}

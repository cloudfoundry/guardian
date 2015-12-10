package ports

import (
	"encoding/json"
	"fmt"
	"os"
)

type State struct {
	Offset uint32 `json:"offset"`
}

func LoadState(filePath string) (State, error) {
	stateFile, err := os.Open(filePath)
	if err != nil {
		return State{}, fmt.Errorf("openning state file: %s", err)
	}
	defer stateFile.Close()

	var state State
	if err := json.NewDecoder(stateFile).Decode(&state); err != nil {
		return State{}, fmt.Errorf("parsing state file: %s", err)
	}

	return state, nil
}

func SaveState(filePath string, state State) error {
	stateFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating state file: %s", err)
	}
	defer stateFile.Close()

	json.NewEncoder(stateFile).Encode(state)
	return nil
}

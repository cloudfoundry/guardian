package ports

import (
	"encoding/json"
	"fmt"
	"os"
)

type State struct {
	Offset uint32 `json:"offset"`
}

type StateFileNotFoundError struct {
	Cause error
}

type StateFileNotValidError struct {
	Cause error
}

func (err StateFileNotFoundError) Error() string {
	return fmt.Sprintf("opening state file caused %s", err.Cause)
}

func (err StateFileNotValidError) Error() string {
	return fmt.Sprintf("parsing state file caused %s", err.Cause)
}

func LoadState(filePath string) (State, error) {
	stateFile, err := os.Open(filePath)
	if err != nil {
		return State{}, StateFileNotFoundError{err}
	}
	defer stateFile.Close()

	var state State
	if err := json.NewDecoder(stateFile).Decode(&state); err != nil {
		return State{}, StateFileNotValidError{err}
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

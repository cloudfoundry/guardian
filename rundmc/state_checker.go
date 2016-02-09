package rundmc

import (
	"encoding/json"
	"os"
	"path"

	"github.com/cloudfoundry-incubator/garden-shed/pkg/retrier"
	"github.com/pivotal-golang/lager"
)

type State struct {
	Pid int `json:"init_process_pid"`
}

type StateChecker struct {
	StateFileDir string
	Retrier      retrier.Retrier
}

func (s StateChecker) State(log lager.Logger, id string) (state State, lastErr error) {
	lastErr = s.Retrier.Retry(func() error {
		state, lastErr = readFromStateFile(log, path.Join(s.StateFileDir, id, "state.json"))
		if lastErr != nil {
			return lastErr
		}
		return nil
	})

	return
}

func readFromStateFile(log lager.Logger, path string) (State, error) {
	log = log.Session("read-state-file", lager.Data{"path": path})
	log.Info("start")
	defer log.Info("finished")

	fd, err := os.Open(path)
	if err != nil {
		log.Error("open-failed", err)
		return State{}, err
	}

	defer fd.Close()

	state := State{}
	if err := json.NewDecoder(fd).Decode(&state); err != nil {
		log.Error("decode-failed", err)
		return State{}, err
	}

	return state, nil
}

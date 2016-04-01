package runrunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/pivotal-golang/lager"
)

type Status string

const RunningStatus Status = "running"

type State struct {
	Pid    int
	Status Status
}

type Stater struct {
	runner RuncCmdRunner
	runc   RuncBinary
}

func NewStater(runner RuncCmdRunner, runc RuncBinary) *Stater {
	return &Stater{
		runner, runc,
	}
}

// State gets the state of the bundle
func (r *Stater) State(log lager.Logger, handle string) (state State, err error) {
	log = log.Session("State", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	buf := new(bytes.Buffer)
	err = r.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		cmd := r.runc.StateCommand(handle, logFile)
		cmd.Stdout = buf
		return cmd
	})

	if err != nil {
		log.Error("state-cmd-failed", err)
		return State{}, fmt.Errorf("runc state: %s", err)
	}

	if err := json.NewDecoder(buf).Decode(&state); err != nil {
		log.Error("decode-state-failed", err)
		return State{}, fmt.Errorf("runc state: %s", err)
	}

	return state, nil
}

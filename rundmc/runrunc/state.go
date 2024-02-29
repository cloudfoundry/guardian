package runrunc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"code.cloudfoundry.org/guardian/rundmc"

	"code.cloudfoundry.org/lager/v3"
)

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
func (r *Stater) State(log lager.Logger, handle string) (state rundmc.State, err error) {
	log = log.Session("state", lager.Data{"handle": handle})

	log.Debug("started")
	defer log.Debug("finished")

	buf := new(bytes.Buffer)
	err = r.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		cmd := r.runc.StateCommand(handle, logFile)
		cmd.Stdout = buf
		return cmd
	})
	if err != nil {
		return rundmc.State{}, fmt.Errorf("runc state: %s", err)
	}

	if err := json.NewDecoder(buf).Decode(&state); err != nil {
		log.Error("decode-state-failed", err)
		return rundmc.State{}, fmt.Errorf("runc state: %s", err)
	}

	return state, nil
}

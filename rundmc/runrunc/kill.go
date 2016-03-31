package runrunc

import (
	"bytes"
	"fmt"

	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/pivotal-golang/lager"
)

type Killer struct {
	commandRunner command_runner.CommandRunner
	runc          RuncBinary
}

func NewKiller(commandRunner command_runner.CommandRunner, runc RuncBinary) *Killer {
	return &Killer{
		commandRunner, runc,
	}
}

// Kill a bundle using 'runc kill'
func (r *Killer) Kill(log lager.Logger, handle string) error {
	log = log.Session("kill", lager.Data{"handle": handle})

	log.Info("started")
	defer log.Info("finished")

	buf := new(bytes.Buffer)
	cmd := r.runc.KillCommand(handle, "KILL")
	cmd.Stderr = buf

	if err := r.commandRunner.Run(cmd); err != nil {
		log.Error("run-failed", err, lager.Data{"stderr": buf.String()})
		return fmt.Errorf("runc kill: %s: %s", err, string(buf.String()))
	}

	return nil
}

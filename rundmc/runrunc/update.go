package runrunc

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Updater struct {
	runner RuncCmdRunner
	runc   RuncBinary
}

func NewUpdater(runner RuncCmdRunner, runc RuncBinary) *Updater {
	return &Updater{
		runner: runner,
		runc:   runc,
	}
}

func (u *Updater) UpdateLimits(log lager.Logger, handle string, limits garden.Limits) error {
	log = log.Session("update-limits", lager.Data{"handle": handle, "limits": limits})

	log.Info("started")
	defer log.Info("finished")

	runcLimits := specs.LinuxResources{
		CPU: &specs.LinuxCPU{
			Quota:  &limits.CPU.Quota,
			Period: &limits.CPU.Period,
		},
	}
	limitsBytes, err := json.Marshal(runcLimits)
	if err != nil {
		return err
	}

	return u.runner.RunAndLog(log, func(logFile string) *exec.Cmd {
		cmd := u.runc.UpdateCommand(handle, logFile)
		cmd.Stdin = bytes.NewReader(limitsBytes)
		log.Error("runc-update-command:", errors.New("foo-error"), lager.Data{"cmd": cmd, "limits": string(limitsBytes)})
		return cmd
	})
}

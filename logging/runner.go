package logging

import (
	"bytes"
	"io"
	"os/exec"
	"syscall"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/gunk/command_runner"
)

type Runner struct {
	command_runner.CommandRunner

	Logger lager.Logger
}

func (runner *Runner) Run(cmd *exec.Cmd) error {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	if cmd.Stdout == nil {
		cmd.Stdout = stdout
	} else {
		cmd.Stdout = io.MultiWriter(cmd.Stdout, stdout)
	}

	if cmd.Stderr == nil {
		cmd.Stderr = stderr
	} else {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, stderr)
	}

	rLog := runner.Logger.Session("command", lager.Data{
		"argv": cmd.Args,
	})

	started := time.Now()

	rLog.Debug("starting")

	err := runner.CommandRunner.Run(cmd)

	data := lager.Data{
		"took": time.Since(started).String(),
	}

	state := cmd.ProcessState
	if state != nil {
		data["exit-status"] = state.Sys().(syscall.WaitStatus).ExitStatus()
	}

	if err != nil {
		data["stdout"] = stdout.String()
		data["stderr"] = stderr.String()

		rLog.Error("failed", err, data)
	} else {
		rLog.Debug("succeeded", data)
	}

	return err
}

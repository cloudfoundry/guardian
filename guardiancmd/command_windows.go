package guardiancmd

import (
	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
)

type NoopExecRunner struct{}

func (n *NoopExecRunner) Run(log lager.Logger, passedID string, spec *runrunc.PreparedSpec, bundlePath, processesPath, handle string, tty *garden.TTYSpec, io garden.ProcessIO) (garden.Process, error) {
	panic("not supported on this platform")
}

func (n *NoopExecRunner) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	panic("not supported on this platform")
}

func commandRunner() commandrunner.CommandRunner {
	return windows_command_runner.New(false)
}

func (cmd *ServerCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	return gardener.NoopVolumeCreator{}
}

func (cmd *ServerCommand) wireExecRunner(dadooPath, runcPath, runcRoot string, processIDGen runrunc.UidGenerator, commandRunner commandrunner.CommandRunner, shouldCleanup bool) runrunc.ExecRunner {
	return &NoopExecRunner{}
}

package guardiancmd

import (
	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
)

type NoopExecRunner struct{}
type NoopStarter struct{}
type NoopChowner struct{}

func (e *NoopExecRunner) Run(log lager.Logger, passedID string, spec *runrunc.PreparedSpec, bundlePath, processesPath, handle string, tty *garden.TTYSpec, io garden.ProcessIO) (garden.Process, error) {
	panic("not supported on this platform")
}

func (e *NoopExecRunner) Attach(log lager.Logger, processID string, io garden.ProcessIO, processesPath string) (garden.Process, error) {
	panic("not supported on this platform")
}

func (s *NoopStarter) Start() error {
	return nil
}

func (c *NoopChowner) Chown(path string, uid, gid int) error {
	return nil
}

func commandRunner() commandrunner.CommandRunner {
	return windows_command_runner.New(false)
}

func wireDepot(depotPath string, bundleSaver depot.BundleSaver) *depot.DirectoryDepot {
	return depot.New(depotPath, bundleSaver, &NoopChowner{})
}

func (cmd *ServerCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	if cmd.Image.Plugin.Path() != "" || cmd.Image.PrivilegedPlugin.Path() != "" {
		return cmd.wireImagePlugin()
	}

	return gardener.NoopVolumeCreator{}
}

func (cmd *ServerCommand) wireExecRunner(dadooPath, runcPath, runcRoot string, processIDGen runrunc.UidGenerator, commandRunner commandrunner.CommandRunner, shouldCleanup bool) runrunc.ExecRunner {
	return &NoopExecRunner{}
}

func (cmd *ServerCommand) wireCgroupsStarter(logger lager.Logger) gardener.Starter {
	return &NoopStarter{}
}

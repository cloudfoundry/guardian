package guardiancmd

import (
	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type NoopStarter struct{}

func (s *NoopStarter) Start() error {
	return nil
}

func commandRunner() commandrunner.CommandRunner {
	return windows_command_runner.New(false)
}

func wireDepot(depotPath string, bundleGenerator depot.BundleGenerator, bundleSaver depot.BundleSaver) *depot.DirectoryDepot {
	return depot.New(depotPath, bundleGenerator, bundleSaver)
}

func (cmd *ServerCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	if cmd.Image.Plugin.Path() != "" || cmd.Image.PrivilegedPlugin.Path() != "" {
		return cmd.wireImagePlugin()
	}

	return gardener.NoopVolumeCreator{}
}

func (cmd *ServerCommand) wireExecRunner(dadooPath, runcPath, runcRoot string, processIDGen runrunc.UidGenerator, commandRunner commandrunner.CommandRunner, shouldCleanup bool) *execrunner.DirectExecRunner {
	return &execrunner.DirectExecRunner{
		RuntimePath:   runcPath,
		CommandRunner: windows_command_runner.New(false),
		ProcessIDGen:  processIDGen,
	}
}

func (cmd *ServerCommand) wireCgroupsStarter(logger lager.Logger) gardener.Starter {
	return &NoopStarter{}
}

func (cmd *ServerCommand) wireExecPreparer() runrunc.ExecPreparer {
	return &runrunc.WindowsExecPreparer{}
}

func defaultBindMounts(binInitPath string) []specs.Mount {
	return []specs.Mount{}
}

func privilegedMounts() []specs.Mount {
	return []specs.Mount{}
}

func unprivilegedMounts() []specs.Mount {
	return []specs.Mount{}
}

func osSpecificBundleRules() []rundmc.BundlerRule {
	return []rundmc.BundlerRule{}
}

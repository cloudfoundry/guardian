package guardiancmd

import (
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type NoopStarter struct{}

func (s *NoopStarter) Start() error {
	return nil
}

type NoopResolvConfigurer struct{}

func (*NoopResolvConfigurer) Configure(log lager.Logger, cfg kawasaki.NetworkConfig, pid int) error {
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

type mkdirer struct{}

func (m mkdirer) MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, paths ...string) error {
	for _, path := range paths {
		volumeName := filepath.VolumeName(path)
		if err := os.MkdirAll(filepath.Join(rootFSPathFile, strings.TrimPrefix(path, volumeName)), 0755); err != nil {
			return err
		}
	}
	return nil
}

func (cmd *ServerCommand) wireExecPreparer() runrunc.ExecPreparer {
	runningAsRoot := func() bool { return true }
	return runrunc.NewExecPreparer(&goci.BndlLoader{}, runrunc.LookupFunc(runrunc.LookupUser), runrunc.EnvFunc(runrunc.WindowsEnvFor), mkdirer{}, nil, runningAsRoot)
}

func wireResolvConfigurer(depotPath string) kawasaki.DnsResolvConfigurer {
	return &NoopResolvConfigurer{}
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

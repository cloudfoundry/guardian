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
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/idmapper"
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

func (cmd *ServerCommand) wireVolumizer(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string, uidMappings, gidMappings idmapper.MappingList) gardener.Volumizer {
	if cmd.Image.Plugin.Path() != "" || cmd.Image.PrivilegedPlugin.Path() != "" {
		return cmd.wireImagePlugin()
	}

	return gardener.NoopVolumizer{}
}

func (cmd *ServerCommand) wireExecRunner(dadooPath, runcPath string, processIDGen runrunc.UidGenerator, commandRunner commandrunner.CommandRunner, shouldCleanup bool) *execrunner.DirectExecRunner {
	return &execrunner.DirectExecRunner{
		RuntimePath:   runcPath,
		CommandRunner: windows_command_runner.New(false),
		ProcessIDGen:  processIDGen,
	}
}

func wireCgroupsStarter(_ lager.Logger, _ string, _ cgroups.Chowner) gardener.Starter {
	return &NoopStarter{}
}

func wireMkdirer(_ commandrunner.CommandRunner) runrunc.Mkdirer {
	return mkdirer{}
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

func wireEnvFunc() runrunc.EnvFunc {
	return runrunc.EnvFunc(runrunc.WindowsEnvFor)
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

func getPrivilegedDevices() []specs.LinuxDevice {
	return nil
}

func mustGetMaxValidUID() int {
	return -1
}

func ensureServerSocketDoesNotLeak(socketFD uintptr) error {
	panic("this should be unreachable: no sockets on Windows")
}

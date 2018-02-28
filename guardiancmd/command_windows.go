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
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type WindowsFactory struct {
	config        *ServerCommand
	commandRunner commandrunner.CommandRunner
}

func (cmd *ServerCommand) NewGardenFactory() GardenFactory {
	return &WindowsFactory{config: cmd, commandRunner: windows_command_runner.New(false)}
}

type NoopStarter struct{}

func (s *NoopStarter) Start() error {
	return nil
}

func (f *WindowsFactory) WireCgroupsStarter(_ lager.Logger) gardener.Starter {
	return &NoopStarter{}
}

func (cmd *SetupCommand) WireCgroupsStarter(_ lager.Logger) gardener.Starter {
	return &NoopStarter{}
}

type NoopResolvConfigurer struct{}

func (*NoopResolvConfigurer) Configure(log lager.Logger, cfg kawasaki.NetworkConfig, pid int) error {
	return nil
}

func (f *WindowsFactory) WireResolvConfigurer() kawasaki.DnsResolvConfigurer {
	return &NoopResolvConfigurer{}
}

func (f *WindowsFactory) WireVolumizer(logger lager.Logger) gardener.Volumizer {
	if f.config.Image.Plugin.Path() != "" || f.config.Image.PrivilegedPlugin.Path() != "" {
		return f.config.wireImagePlugin(f.commandRunner, 0, 0)
	}

	noop := gardener.NoopVolumizer{}
	return gardener.NewVolumeProvider(noop, noop, gardener.CommandFactory(preparerootfs.Command), f.commandRunner, 0, 0)
}

func (f *WindowsFactory) WireExecRunner(runMode string) runrunc.ExecRunner {
	return &execrunner.DirectExecRunner{
		RuntimePath:   f.config.Runtime.Plugin,
		CommandRunner: f.commandRunner,
		RunMode:       runMode,
	}
}

func (f *WindowsFactory) WireRootfsFileCreator() rundmc.RootfsFileCreator {
	return noopRootfsFileCreator{}
}

type noopRootfsFileCreator struct{}

func (noopRootfsFileCreator) CreateFiles(rootFSPath string, pathsToCreate ...string) error {
	return nil
}

func (f *WindowsFactory) CommandRunner() commandrunner.CommandRunner {
	return f.commandRunner
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

func (f *WindowsFactory) WireMkdirer() runrunc.Mkdirer {
	return mkdirer{}
}

func wireEnvFunc() runrunc.EnvFunc {
	return runrunc.EnvFunc(runrunc.WindowsEnvFor)
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

func getPrivilegedDevices() []specs.LinuxDevice {
	return nil
}

func bindMountPoints() []string {
	return nil
}

func mustGetMaxValidUID() int {
	return -1
}

func ensureServerSocketDoesNotLeak(socketFD uintptr) error {
	panic("this should be unreachable: no sockets on Windows")
}

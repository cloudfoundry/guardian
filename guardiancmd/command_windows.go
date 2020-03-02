package guardiancmd

import (
	"errors"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/processes"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/privchecker"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/lager"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type WindowsFactory struct {
	config        *CommonCommand
	commandRunner commandrunner.CommandRunner
}

func (cmd *CommonCommand) NewGardenFactory() GardenFactory {
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

type NoopService struct{}

func (NoopService) Start() {}
func (NoopService) Stop()  {}

func (f *WindowsFactory) WireResolvConfigurer() kawasaki.DnsResolvConfigurer {
	return &NoopResolvConfigurer{}
}

func (f *WindowsFactory) WireVolumizer(logger lager.Logger) gardener.Volumizer {
	if f.config.Image.Plugin.Path() != "" || f.config.Image.PrivilegedPlugin.Path() != "" {
		return f.config.wireImagePlugin(f.commandRunner, 0, 0)
	}

	noop := gardener.NoopVolumizer{}
	return gardener.NewVolumeProvider(noop, noop, f.commandRunner, 0, 0)
}

func (f *WindowsFactory) WireExecRunner(runcRoot string, _, _ uint32, bundleSaver depot.BundleSaver, bundleLookupper depot.BundleLookupper, processDepot execrunner.ProcessDepot) runrunc.ExecRunner {
	return execrunner.NewWindowsExecRunner(f.config.Runtime.Plugin, f.commandRunner, bundleSaver, bundleLookupper, processDepot)
}

func (f *WindowsFactory) WireRootfsFileCreator() depot.RootfsFileCreator {
	return noopRootfsFileCreator{}
}

type noopRootfsFileCreator struct{}

func (noopRootfsFileCreator) CreateFiles(rootFSPath string, pathsToCreate ...string) error {
	return nil
}

func (f *WindowsFactory) CommandRunner() commandrunner.CommandRunner {
	return f.commandRunner
}

func (f *WindowsFactory) WireContainerd(processBuilder *processes.ProcBuilder, userLookupper users.UserLookupper, wireExecer func(pidGetter runrunc.PidGetter) *runrunc.Execer, statser runcontainerd.Statser, log lager.Logger, volumizer peas.Volumizer, peaHandlesGetter runcontainerd.PeaHandlesGetter) (*runcontainerd.RunContainerd, *runcontainerd.RunContainerPea, *runcontainerd.PidGetter, *privchecker.PrivilegeChecker, peas.BundleLoader, error) {
	return nil, nil, nil, nil, nil, errors.New("containerd not impletemented on windows")
}

func (f *WindowsFactory) WireCPUCgrouper() (rundmc.CPUCgrouper, error) {
	return cgroups.NoopCPUCgrouper{}, nil
}

func wireEnvFunc() processes.EnvFunc {
	return processes.WindowsEnvFor
}

func wireMounts() bundlerules.Mounts {
	noopMountOptionsGetter := func(path string) ([]string, error) {
		return []string{}, nil
	}
	return bundlerules.Mounts{MountOptionsGetter: noopMountOptionsGetter}
}

// Note - it's not possible to bind mount a single file in Windows, so we are
// using a directory instead
func initBindMountAndPath(initPathOnHost string) (specs.Mount, string) {
	initDirInContainer := filepath.Join(`C:\`, "Windows", "Temp", "bin", "init")
	initPathInContainer := filepath.Join(initDirInContainer, filepath.Base(initPathOnHost))
	return specs.Mount{
		Type:        "bind",
		Source:      filepath.Dir(initPathOnHost),
		Destination: initDirInContainer,
		Options:     []string{"bind"},
	}, initPathInContainer
}

func mkdirerBindMountAndPath(mkdirerPathOnHost string) (specs.Mount, string) {
	mkdirerDirInContainer := filepath.Join(`C:\`, "Windows", "Temp", "bin", "mkdir")
	mkdirerPathInContainer := filepath.Join(mkdirerDirInContainer, filepath.Base(mkdirerPathOnHost))
	return specs.Mount{
		Type:        "bind",
		Source:      filepath.Dir(mkdirerPathOnHost),
		Destination: mkdirerDirInContainer,
		Options:     []string{"bind"},
	}, mkdirerPathInContainer
}

func defaultBindMounts() []specs.Mount {
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

func containerdRuncRoot() string {
	return ""
}

func (cmd *CommonCommand) wireKernelParams() []rundmc.BundlerRule {
	return []rundmc.BundlerRule{}
}

func (cmd *CommonCommand) computeRuncRoot() string {
	return ""
}

func (cmd *CommonCommand) wireCpuThrottlingService(log lager.Logger, containerizer *rundmc.Containerizer, memoryProvider throttle.MemoryProvider) (Service, error) {
	return &NoopService{}, nil
}

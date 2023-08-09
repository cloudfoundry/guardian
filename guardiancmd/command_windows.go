package guardiancmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

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
	"code.cloudfoundry.org/lager/v3"
	"github.com/opencontainers/runtime-spec/specs-go"
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

func (f *WindowsFactory) WireContainerd(processBuilder *processes.ProcBuilder, userLookupper users.UserLookupper, wireExecer func(pidGetter runrunc.PidGetter) *runrunc.Execer, statser runcontainerd.Statser, log lager.Logger, volumizer peas.Volumizer, peaHandlesGetter runcontainerd.PeaHandlesGetter) (*runcontainerd.RunContainerd, *runcontainerd.RunContainerPea, *runcontainerd.PidGetter, *privchecker.PrivilegeChecker, peas.BundleLoader, error) {
	return nil, nil, nil, nil, nil, errors.New("containerd not impletemented on windows")
}

func (f *WindowsFactory) WireCPUCgrouper() (rundmc.CPUCgrouper, error) {
	return cgroups.NoopCPUCgrouper{}, nil
}

func (f *WindowsFactory) WireContainerNetworkMetricsProvider(_ gardener.Containerizer, _ gardener.PropertyManager) gardener.ContainerNetworkMetricsProvider {
	return gardener.NewNoopContainerNetworkMetricsProvider()
}

func wireEnvFunc() processes.EnvFunc {
	return processes.WindowsEnvFor
}

func wireMounts(logger lager.Logger) bundlerules.Mounts {
	noopMountOptionsGetter := func(path string) ([]string, error) {
		return []string{}, nil
	}
	return bundlerules.NewMounts(logger, noopMountOptionsGetter)
}

// Note - it's not possible to bind mount a single file in Windows, so we are
// using a directory instead
func initBindMountAndPath(initPathOnHost string) (specs.Mount, string) {
	initPathInContainer := filepath.Join(`C:\`, "Windows", "Temp", "bin", filepath.Base(initPathOnHost))
	return specs.Mount{
		Type:        "bind",
		Source:      filepath.Dir(initPathOnHost),
		Destination: filepath.Dir(initPathInContainer),
		Options:     []string{"bind"},
	}, initPathInContainer
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

func (cmd *CommonCommand) wireCpuThrottlingService(log lager.Logger, containerizer *rundmc.Containerizer, memoryProvider throttle.MemoryProvider, cpuEntitlementPerShare float64) (Service, error) {
	return &NoopService{}, nil
}

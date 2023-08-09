package guardiancmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/guardiancmd/cpuentitlement"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	"code.cloudfoundry.org/guardian/rundmc/processes"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	nerdpkg "code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/privchecker"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/signals"
	"code.cloudfoundry.org/guardian/rundmc/sysctl"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/sysinfo"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager/v3"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/process"
	"github.com/containerd/containerd/plugin"
	cgrouputils "github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

type LinuxFactory struct {
	config           *CommonCommand
	commandRunner    commandrunner.CommandRunner
	signallerFactory *signals.SignallerFactory
	uidMappings      idmapper.MappingList
	gidMappings      idmapper.MappingList
}

func (cmd *CommonCommand) NewGardenFactory() GardenFactory {
	uidMappings, gidMappings := cmd.idMappings()
	return &LinuxFactory{
		config:           cmd,
		commandRunner:    linux_command_runner.New(),
		signallerFactory: &signals.SignallerFactory{PidGetter: wirePidfileReader()},
		uidMappings:      uidMappings,
		gidMappings:      gidMappings,
	}
}

func (f *LinuxFactory) CommandRunner() commandrunner.CommandRunner {
	return f.commandRunner
}

func (f *LinuxFactory) WireVolumizer(logger lager.Logger) gardener.Volumizer {
	if f.config.Image.Plugin.Path() != "" || f.config.Image.PrivilegedPlugin.Path() != "" {
		return f.config.wireImagePlugin(f.commandRunner, f.uidMappings.Map(0), f.gidMappings.Map(0))
	}

	noop := gardener.NoopVolumizer{}
	return gardener.NewVolumeProvider(noop, noop, f.commandRunner, f.uidMappings.Map(0), f.gidMappings.Map(0))
}

func wireEnvFunc() processes.EnvFunc {
	return processes.UnixEnvFor
}

func (f *LinuxFactory) WireMkdirer() runrunc.Mkdirer {
	if runningAsRoot() {
		return bundlerules.MkdirChowner{Command: preparerootfs.Command, CommandRunner: f.commandRunner}
	}

	return NoopMkdirer{}
}

type NoopMkdirer struct{}

func (NoopMkdirer) MkdirAs(rootFSPathFile string, uid, gid int, mode os.FileMode, recreate bool, path ...string) error {
	return nil
}

func (f *LinuxFactory) WireExecRunner(runcRoot string, containerRootHostUID, containerRootHostGID uint32, bundleSaver depot.BundleSaver, bundleLookupper depot.BundleLookupper, processDepot execrunner.ProcessDepot) runrunc.ExecRunner {
	return dadoo.NewExecRunner(
		f.config.Bin.Dadoo.Path(),
		f.config.Runtime.Plugin,
		runcRoot,
		f.signallerFactory,
		f.commandRunner,
		f.config.Containers.CleanupProcessDirsOnWait,
		containerRootHostUID,
		containerRootHostGID,
		bundleSaver,
		bundleLookupper,
		processDepot,
	)
}

func (f *LinuxFactory) WireCgroupsStarter(logger lager.Logger) gardener.Starter {
	return createCgroupsStarter(logger, f.config.Server.Tag, rundmc.IsMountPoint, f.config.CPUThrottling.Enabled)
}

func (cmd *SetupCommand) WireCgroupsStarter(logger lager.Logger) gardener.Starter {
	starter := createCgroupsStarter(logger, cmd.Tag, rundmc.IsMountPoint, cmd.EnableCPUThrottling)

	if cmd.RootlessUID != nil {
		starter = starter.WithUID(*cmd.RootlessUID)
	}

	if cmd.RootlessGID != nil {
		starter = starter.WithGID(*cmd.RootlessGID)
	}

	return starter
}

func createCgroupsStarter(logger lager.Logger, tag string, mountPointChecker rundmc.MountPointChecker, cpuThrottlingEnabled bool) *cgroups.CgroupStarter {
	cgroupsMountpoint := cgroups.Root
	gardenCgroup := cgroups.Garden

	if tag != "" {
		cgroupsMountpoint = filepath.Join("/tmp", fmt.Sprintf("cgroups-%s", tag))
		gardenCgroup = fmt.Sprintf("%s-%s", gardenCgroup, tag)
	}

	return cgroups.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"),
		cgroupsMountpoint, gardenCgroup, allowedDevices, mountPointChecker, cpuThrottlingEnabled)
}

func (f *LinuxFactory) WireResolvConfigurer() kawasaki.DnsResolvConfigurer {
	return &kawasaki.ResolvConfigurer{
		HostsFileCompiler: &dns.HostsFileCompiler{},
		ResolvCompiler:    &dns.ResolvCompiler{},
		ResolvFilePath:    "/etc/resolv.conf",
		DepotDir:          f.config.Containers.Dir,
	}
}

func (f *LinuxFactory) WireContainerd(processBuilder *processes.ProcBuilder, userLookupper users.UserLookupper, wireExecer func(pidGetter runrunc.PidGetter) *runrunc.Execer, statser runcontainerd.Statser, log lager.Logger, volumizer peas.Volumizer, peaHandlesGetter runcontainerd.PeaHandlesGetter) (*runcontainerd.RunContainerd, *runcontainerd.RunContainerPea, *runcontainerd.PidGetter, *privchecker.PrivilegeChecker, peas.BundleLoader, error) {
	containerdClient, err := containerd.New(f.config.Containerd.Socket, containerd.WithDefaultRuntime(plugin.RuntimeRuncV2))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	ctx := namespaces.WithNamespace(context.Background(), containerdNamespace)
	ctx = leases.WithLease(ctx, "lease-is-off")
	nerd := nerdpkg.New(containerdClient, ctx, filepath.Join(containerdRuncRoot(), "fifo"))
	nerdStopper := nerdpkg.NewNerdStopper(containerdClient)
	pidGetter := &runcontainerd.PidGetter{Nerd: nerd}

	cgroupManager := runcontainerd.NewCgroupManager(containerdRuncRoot(), containerdNamespace)

	containerdManager := runcontainerd.New(nerd, nerd, processBuilder, userLookupper, wireExecer(pidGetter), statser, f.config.Containerd.UseContainerdForProcesses, cgroupManager, f.WireMkdirer(), peaHandlesGetter, f.config.Containers.CleanupProcessDirsOnWait, nerdStopper)

	peaRunner := runcontainerd.NewRunContainerPea(containerdManager, nerd, volumizer, f.config.Containers.CleanupProcessDirsOnWait)

	privilegeChecker := &privchecker.PrivilegeChecker{ContainerManager: nerd, Log: log}
	peaBundleLoader := runcontainerd.NewBndlLoader(nerd)

	return containerdManager, peaRunner, pidGetter, privilegeChecker, peaBundleLoader, nil
}

func (f *LinuxFactory) WireCPUCgrouper() (rundmc.CPUCgrouper, error) {
	if !f.config.CPUThrottling.Enabled {
		return cgroups.NoopCPUCgrouper{}, nil
	}

	gardenCPUCgroupPath, err := f.config.getGardenCPUCgroup()
	if err != nil {
		return nil, err
	}
	return cgroups.NewCPUCgrouper(gardenCPUCgroupPath), nil
}

func (f *LinuxFactory) WireContainerNetworkMetricsProvider(containerizer gardener.Containerizer, propertyManager gardener.PropertyManager) gardener.ContainerNetworkMetricsProvider {
	if !f.config.Network.EnableMetrics {
		return gardener.NewNoopContainerNetworkMetricsProvider()
	}

	return gardener.NewSysFSContainerNetworkMetricsProvider(containerizer, propertyManager)
}

func initBindMountAndPath(initPathOnHost string) (specs.Mount, string) {
	initPathInContainer := filepath.Join("/tmp", "garden-init")
	return specs.Mount{
		Type:        "bind",
		Source:      initPathOnHost,
		Destination: initPathInContainer,
		Options:     []string{"bind"},
	}, initPathInContainer
}

func defaultBindMounts() []specs.Mount {
	devptsGid := 0
	if runningAsRoot() {
		devptsGid = 5
	}

	return []specs.Mount{
		{Destination: "/sys", Type: "sysfs", Source: "sysfs", Options: []string{"noexec", "nosuid", "nodev", "ro"}},
		{Destination: "/dev", Type: "tmpfs", Source: "tmpfs", Options: []string{"noexec", "strictatime", "mode=755"}},
		{Destination: "/dev/shm", Type: "tmpfs", Source: "tmpfs", Options: []string{"noexec", "nosuid", "nodev", "mode=1777"}},
		{Destination: "/dev/mqueue", Type: "mqueue", Source: "mqueue", Options: []string{"noexec", "nosuid", "nodev"}},
		{
			Destination: "/dev/pts", Type: "devpts", Source: "devpts",
			Options: []string{"nosuid", "noexec", "newinstance", fmt.Sprintf("gid=%d", devptsGid), "ptmxmode=0666", "mode=0620"},
		},
	}
}

func privilegedMounts() []specs.Mount {
	return []specs.Mount{
		{Destination: "/proc", Type: "proc", Source: "proc", Options: []string{"nosuid", "noexec", "nodev"}},
	}
}

func unprivilegedMounts() []specs.Mount {
	return []specs.Mount{
		{Destination: "/proc", Type: "proc", Source: "proc", Options: []string{"nosuid", "noexec", "nodev"}},
		{Destination: "/sys/fs/cgroup", Type: "cgroup", Source: "cgroup", Options: []string{"ro", "nosuid", "noexec", "nodev"}},
	}
}

func getPrivilegedDevices() []specs.LinuxDevice {
	return []specs.LinuxDevice{fuseDevice}
}

func bindMountPoints() []string {
	return []string{"/etc/hosts", "/etc/resolv.conf"}
}

func mustGetMaxValidUID() int {
	return idmapper.MustGetMaxValidUID()
}

func ensureServerSocketDoesNotLeak(socketFD uintptr) error {
	_, _, errNo := unix.Syscall(unix.SYS_FCNTL, socketFD, unix.F_SETFD, unix.FD_CLOEXEC)
	if errNo != 0 {
		return fmt.Errorf("setting cloexec on server socket: %s", errNo)
	}
	return nil
}

func wireMounts(logger lager.Logger) bundlerules.Mounts {
	return bundlerules.NewMounts(logger, bundlerules.UnprivilegedMountFlagsGetter)
}

func (cmd *CommonCommand) wireKernelParams() []rundmc.BundlerRule {
	sysctl := sysctl.New()

	return []rundmc.BundlerRule{
		bundlerules.NewKernelParameter(sysctl, "net.ipv4.tcp_keepalive_time", cmd.Sysctl.TCPKeepaliveTime),
		bundlerules.NewKernelParameter(sysctl, "net.ipv4.tcp_keepalive_intvl", cmd.Sysctl.TCPKeepaliveInterval),
		bundlerules.NewKernelParameter(sysctl, "net.ipv4.tcp_keepalive_probes", cmd.Sysctl.TCPKeepaliveProbes),
		bundlerules.NewKernelParameter(sysctl, "net.ipv4.tcp_retries1", cmd.Sysctl.TCPRetries1),
		bundlerules.NewKernelParameter(sysctl, "net.ipv4.tcp_retries2", cmd.Sysctl.TCPRetries2),
	}
}

func containerdRuncRoot() string {
	if root := getRuntimeDir(); root != "" {
		return root
	}
	return process.RuncRoot
}

func (cmd *CommonCommand) computeRuncRoot() string {
	if cmd.useContainerd() {
		return filepath.Join(containerdRuncRoot(), containerdNamespace)
	}

	if root := getRuntimeDir(); root != "" {
		return root
	}

	return filepath.Join("/", "run", "runc")
}

func getRuntimeDir() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if os.Geteuid() != 0 && runtimeDir != "" {
		return filepath.Join(runtimeDir, "runc")
	}
	return ""
}

func (cmd *CommonCommand) getGardenCPUCgroup() (string, error) {
	cpuCgroupSubPath, err := cgrouputils.ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}

	cgroupsMountpoint := gardencgroups.Root
	gardenCgroup := gardencgroups.Garden

	if cmd.Server.Tag != "" {
		cgroupsMountpoint = filepath.Join("/tmp", fmt.Sprintf("cgroups-%s", cmd.Server.Tag))
		gardenCgroup = fmt.Sprintf("%s-%s", gardenCgroup, cmd.Server.Tag)
	}

	return filepath.Join(cgroupsMountpoint, "cpu", cpuCgroupSubPath["cpu"], gardenCgroup), nil
}

func (cmd *CommonCommand) wireCpuThrottlingService(log lager.Logger, containerizer *rundmc.Containerizer, memoryProvider throttle.MemoryProvider, cpuEntitlementPerShare float64) (Service, error) {
	metricsSource := throttle.NewContainerMetricsSource(containerizer)
	gardenCPUCgroup, err := cmd.getGardenCPUCgroup()
	if err != nil {
		return nil, err
	}

	sysInfoProvider := sysinfo.NewResourcesProvider(cmd.Containers.Dir)
	cpuEntitlementCalculator := cpuentitlement.Calculator{SysInfoProvider: sysInfoProvider}
	sharesMultiplier, err := cpuEntitlementCalculator.CalculateEntitlementMultiplier(cpuEntitlementPerShare)
	if err != nil {
		log.Error("failed-to-compute-cpu-share-multiplier", err)
		return nil, err
	}

	enforcer := throttle.NewEnforcer(gardenCPUCgroup)
	throttler := throttle.NewThrottler(metricsSource, enforcer)
	sharesBalancer := throttle.NewSharesBalancer(gardenCPUCgroup, memoryProvider, sharesMultiplier)

	if cmd.CPUThrottling.CheckInterval == 0 {
		return nil, errors.New("non-positive CPU throttling checking interval")
	}
	ticker := time.NewTicker(time.Duration(cmd.CPUThrottling.CheckInterval) * time.Second)

	return throttle.NewPollingService(log, throttle.NewCompositeRunnable(throttler, sharesBalancer), ticker.C), nil
}

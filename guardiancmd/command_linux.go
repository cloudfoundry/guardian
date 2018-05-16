package guardiancmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/nerd"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/signals"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/linux/proc"
	"github.com/containerd/containerd/namespaces"
	"github.com/docker/docker/pkg/mount"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

type LinuxFactory struct {
	config           *ServerCommand
	commandRunner    commandrunner.CommandRunner
	signallerFactory *signals.SignallerFactory
	uidMappings      idmapper.MappingList
	gidMappings      idmapper.MappingList
}

func (cmd *ServerCommand) NewGardenFactory() GardenFactory {
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

	if f.config.Graph.Dir == "" {
		noop := gardener.NoopVolumizer{}
		return gardener.NewVolumeProvider(noop, noop, gardener.CommandFactory(preparerootfs.Command), f.commandRunner, f.uidMappings.Map(0), f.gidMappings.Map(0))
	}

	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger}
	shed := rootfs_provider.Wire(
		logger,
		runner,
		f.config.Graph.Dir,
		f.config.Containers.DefaultRootFS,
		f.config.Docker.Registry,
		f.config.Docker.InsecureRegistries,
		f.config.Graph.PersistentImages,
		f.config.Graph.CleanupThresholdInMegabytes,
		f.uidMappings,
		f.gidMappings,
	)
	return gardener.NewVolumeProvider(shed, shed, gardener.CommandFactory(preparerootfs.Command), f.commandRunner, f.uidMappings.Map(0), f.gidMappings.Map(0))
}

func wireEnvFunc() runrunc.EnvFunc {
	return runrunc.EnvFunc(runrunc.UnixEnvFor)
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

func (f *LinuxFactory) WireExecRunner(runMode, runcRoot string) runrunc.ExecRunner {
	return dadoo.NewExecRunner(
		f.config.Bin.Dadoo.Path(),
		f.config.Runtime.Plugin,
		runcRoot,
		f.signallerFactory,
		f.commandRunner,
		f.config.Containers.CleanupProcessDirsOnWait,
		runMode,
	)
}

func (f *LinuxFactory) WireCgroupsStarter(logger lager.Logger) gardener.Starter {
	return createCgroupsStarter(logger, f.config.Server.Tag, "", &cgroups.OSChowner{}, rundmc.IsMountPoint)
}

func (cmd *SetupCommand) WireCgroupsStarter(logger lager.Logger) gardener.Starter {
	return createCgroupsStarter(logger, cmd.Tag, cmd.CgroupRoot, &cgroups.OSChowner{UID: cmd.RootlessUID, GID: cmd.RootlessGID}, rundmc.IsMountPoint)
}

func createCgroupsStarter(logger lager.Logger, tag, cgroupRoot string, chowner cgroups.Chowner, mountPointChecker rundmc.MountPointChecker) gardener.Starter {
	cgroupsMountpoint := cgroups.CgroupRoot
	gardenCgroup := cgroups.GardenCgroup
	if tag != "" {
		gardenCgroup = fmt.Sprintf("%s-%s", gardenCgroup, tag)
	}
	if cgroupRoot != "" {
		cgroupsMountpoint = cgroupRoot
	}

	return cgroups.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"),
		cgroupsMountpoint, gardenCgroup, allowedDevices, linux_command_runner.New(), chowner, mountPointChecker)
}

func (f *LinuxFactory) WireResolvConfigurer() kawasaki.DnsResolvConfigurer {
	return &kawasaki.ResolvConfigurer{
		HostsFileCompiler: &dns.HostsFileCompiler{},
		ResolvCompiler:    &dns.ResolvCompiler{},
		ResolvFilePath:    "/etc/resolv.conf",
		DepotDir:          f.config.Containers.Dir,
	}
}

func (f *LinuxFactory) WireRootfsFileCreator() rundmc.RootfsFileCreator {
	return preparerootfs.SymlinkRefusingFileCreator{}
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
		{Type: "sysfs", Source: "sysfs", Destination: "/sys", Options: []string{"nosuid", "noexec", "nodev", "ro"}},
		{Type: "tmpfs", Source: "tmpfs", Destination: "/dev/shm", Options: []string{"rw", "nodev", "relatime"}},
		{Type: "devpts", Source: "devpts", Destination: "/dev/pts",
			Options: []string{"nosuid", "noexec", "newinstance", fmt.Sprintf("gid=%d", devptsGid), "ptmxmode=0666", "mode=0620"}},
	}
}

func privilegedMounts() []specs.Mount {
	return []specs.Mount{
		{Type: "proc", Source: "proc", Destination: "/proc", Options: []string{"nosuid", "noexec", "nodev"}},
	}
}

func unprivilegedMounts() []specs.Mount {
	return []specs.Mount{
		{Type: "proc", Source: "proc", Destination: "/proc", Options: []string{"nosuid", "noexec", "nodev"}},
		{Type: "cgroup", Source: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"ro", "nosuid", "noexec", "nodev"}},
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

func wireMounts() bundlerules.Mounts {
	return bundlerules.Mounts{
		MountOptionsGetter: rundmc.GetMountOptions,
		MountInfosProvider: func() ([]*mount.Info, error) {
			return mount.GetMounts()
		},
	}
}

func wireContainerd(socket string, bndlLoader *goci.BndlLoader, wireExecer func(pidGetter runrunc.PidGetter) *runrunc.Execer, statser runcontainerd.Statser) (rundmc.OCIRuntime, peas.PidGetter, error) {
	containerdClient, err := containerd.New(socket)
	if err != nil {
		return nil, nil, err
	}
	ctx := namespaces.WithNamespace(context.Background(), containerdNamespace)
	nerd := nerd.New(containerdClient, ctx)
	pidGetter := &runcontainerd.PidGetter{Nerd: nerd}

	return runcontainerd.New(nerd, bndlLoader, wireExecer(pidGetter), statser), pidGetter, nil
}

func containerdRuncRoot() string {
	return proc.RuncRoot
}

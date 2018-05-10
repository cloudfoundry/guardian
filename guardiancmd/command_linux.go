package guardiancmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/garden-shed/distclient"
	quotaed_aufs "code.cloudfoundry.org/garden-shed/docker_drivers/aufs"
	"code.cloudfoundry.org/garden-shed/layercake"
	"code.cloudfoundry.org/garden-shed/layercake/cleaner"
	"code.cloudfoundry.org/garden-shed/quota_manager"
	"code.cloudfoundry.org/garden-shed/repository_fetcher"
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
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/mount"
	"github.com/eapache/go-resiliency/retrier"
	specs "github.com/opencontainers/runtime-spec/specs-go"
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

	shed := f.wireShed(logger)
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
	_, _, errNo := syscall.Syscall(syscall.SYS_FCNTL, socketFD, syscall.F_SETFD, syscall.FD_CLOEXEC)
	if errNo != 0 {
		return fmt.Errorf("setting cloexec on server socket: %s", errNo)
	}
	return nil
}

func (f *LinuxFactory) wireShed(logger lager.Logger) *rootfs_provider.CakeOrdinator {
	graphRoot := f.config.Graph.Dir
	logger = logger.Session(gardener.VolumizerSession, lager.Data{"graphRoot": graphRoot})

	if err := exec.Command("modprobe", "aufs").Run(); err != nil {
		logger.Error("unable-to-load-aufs", err)
	}

	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger}

	if err := os.MkdirAll(graphRoot, 0755); err != nil {
		logger.Fatal("failed-to-create-graph-directory", err)
	}

	dockerGraphDriver, err := graphdriver.New(graphRoot, nil)
	if err != nil {
		logger.Fatal("failed-to-construct-graph-driver", err)
	}

	backingStoresPath := filepath.Join(graphRoot, "backing_stores")
	if mkdirErr := os.MkdirAll(backingStoresPath, 0660); mkdirErr != nil {
		logger.Fatal("failed-to-mkdir-backing-stores", mkdirErr)
	}

	quotaedGraphDriver := &quotaed_aufs.QuotaedDriver{
		GraphDriver: dockerGraphDriver,
		Unmount:     quotaed_aufs.Unmount,
		BackingStoreMgr: &quotaed_aufs.BackingStore{
			RootPath: backingStoresPath,
			Logger:   logger.Session("backing-store-mgr"),
		},
		LoopMounter: &quotaed_aufs.Loop{
			Retrier: retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil),
			Logger:  logger.Session("loop-mounter"),
		},
		Retrier:  retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil),
		RootPath: graphRoot,
		Logger:   logger.Session("quotaed-driver"),
	}

	dockerGraph, err := graph.NewGraph(graphRoot, quotaedGraphDriver)
	if err != nil {
		logger.Fatal("failed-to-construct-graph", err)
	}

	var cake layercake.Cake = &layercake.Docker{
		Graph:  dockerGraph,
		Driver: quotaedGraphDriver,
	}

	if cake.DriverName() == "aufs" {
		cake = &layercake.AufsCake{
			Cake:      cake,
			Runner:    runner,
			GraphRoot: graphRoot,
		}
	}

	repoFetcher := repository_fetcher.Retryable{
		RepositoryFetcher: &repository_fetcher.CompositeFetcher{
			LocalFetcher: &repository_fetcher.Local{
				Cake:              cake,
				DefaultRootFSPath: f.config.Containers.DefaultRootFS,
				IDProvider:        repository_fetcher.LayerIDProvider{},
			},
			RemoteFetcher: repository_fetcher.NewRemote(
				f.config.Docker.Registry,
				cake,
				distclient.NewDialer(f.config.Docker.InsecureRegistries),
				repository_fetcher.VerifyFunc(repository_fetcher.Verify),
			),
		},
	}

	rootFSNamespacer := &rootfs_provider.UidNamespacer{
		Translator: rootfs_provider.NewUidTranslator(
			f.uidMappings,
			f.gidMappings,
		),
	}

	retainer := cleaner.NewRetainer()
	ovenCleaner := cleaner.NewOvenCleaner(retainer,
		cleaner.NewThreshold(int64(f.config.Graph.CleanupThresholdInMegabytes)*1024*1024),
	)

	imageRetainer := &repository_fetcher.ImageRetainer{
		GraphRetainer:             retainer,
		DirectoryRootfsIDProvider: repository_fetcher.LayerIDProvider{},
		DockerImageIDFetcher:      repoFetcher,

		NamespaceCacheKey: rootFSNamespacer.CacheKey(),
		Logger:            logger,
	}

	// spawn off in a go function to avoid blocking startup
	// worst case is if an image is immediately created and deleted faster than
	// we can retain it we'll garbage collect it when we shouldn't. This
	// is an OK trade-off for not having garden startup block on dockerhub.
	go imageRetainer.Retain(f.config.Graph.PersistentImages)

	layerCreator := rootfs_provider.NewLayerCreator(cake, rootfs_provider.SimpleVolumeCreator{}, rootFSNamespacer)

	quotaManager := &quota_manager.AUFSQuotaManager{
		BaseSizer: quota_manager.NewAUFSBaseSizer(cake),
		DiffSizer: &quota_manager.AUFSDiffSizer{
			AUFSDiffPathFinder: quotaedGraphDriver,
		},
	}

	return rootfs_provider.NewCakeOrdinator(cake,
		repoFetcher,
		layerCreator,
		rootfs_provider.NewMetricsAdapter(quotaManager.GetUsage, quotaedGraphDriver.GetMntPath),
		ovenCleaner)
}

func wireMounts() bundlerules.Mounts {
	return bundlerules.Mounts{
		MountOptionsGetter: rundmc.GetMountOptions,
		MountInfosProvider: func() ([]*mount.Info, error) {
			return mount.GetMounts()
		},
	}
}

func wireContainerd(socket string, bndlLoader *goci.BndlLoader, wireExecer func(pidGetter runrunc.PidGetter) *runrunc.Execer) (rundmc.OCIRuntime, error) {
	containerdClient, err := containerd.New(socket)
	if err != nil {
		return nil, err
	}
	ctx := namespaces.WithNamespace(context.Background(), containerdNamespace)
	nerd := nerd.New(containerdClient, ctx)
	return runcontainerd.New(nerd, bndlLoader, wireExecer(&runcontainerd.PidGetter{Nerd: nerd})), nil
}

func containerdRuncRoot() string {
	return proc.RuncRoot
}

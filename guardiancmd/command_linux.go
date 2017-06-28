package guardiancmd

import (
	"fmt"
	"os"
	"path/filepath"
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
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/idmapper"
	"code.cloudfoundry.org/lager"
	"github.com/docker/docker/daemon/graphdriver"
	"github.com/docker/docker/graph"
	"github.com/eapache/go-resiliency/retrier"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pivotal-golang/clock"
)

func init() {
	maxUID := uint32(idmapper.MustGetMaxValidUID())
	maxGID := uint32(idmapper.MustGetMaxValidGID())
	uidMappings = idmapper.MappingList{
		{
			ContainerID: 0,
			HostID:      maxUID,
			Size:        1,
		},
		{
			ContainerID: 1,
			HostID:      1,
			Size:        maxUID - 1,
		},
	}

	gidMappings = idmapper.MappingList{
		{
			ContainerID: 0,
			HostID:      maxGID,
			Size:        1,
		},
		{
			ContainerID: 1,
			HostID:      1,
			Size:        maxGID - 1,
		},
	}
}

func commandRunner() commandrunner.CommandRunner {
	return linux_command_runner.New()
}

func wireDepot(depotPath string, bundler depot.BundleGenerator, bundleSaver depot.BundleSaver) *depot.DirectoryDepot {
	return depot.New(depotPath, bundler, bundleSaver)
}

func (cmd *ServerCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	if graphRoot == "" {
		return gardener.NoopVolumeCreator{}
	}

	if cmd.Image.Plugin.Path() != "" || cmd.Image.PrivilegedPlugin.Path() != "" {
		return cmd.wireImagePlugin()
	}

	logger = logger.Session("volume-creator", lager.Data{"graphRoot": graphRoot})
	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger}

	if err := os.MkdirAll(graphRoot, 0755); err != nil {
		logger.Fatal("failed-to-create-graph-directory", err)
	}

	dockerGraphDriver, err := graphdriver.New(graphRoot, nil)
	if err != nil {
		logger.Fatal("failed-to-construct-graph-driver", err)
	}

	backingStoresPath := filepath.Join(graphRoot, "backing_stores")
	if err := os.MkdirAll(backingStoresPath, 0660); err != nil {
		logger.Fatal("failed-to-mkdir-backing-stores", err)
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
				DefaultRootFSPath: cmd.Containers.DefaultRootFS,
				IDProvider:        repository_fetcher.LayerIDProvider{},
			},
			RemoteFetcher: repository_fetcher.NewRemote(
				cmd.Docker.Registry,
				cake,
				distclient.NewDialer(insecureRegistries),
				repository_fetcher.VerifyFunc(repository_fetcher.Verify),
			),
		},
	}

	rootFSNamespacer := &rootfs_provider.UidNamespacer{
		Translator: rootfs_provider.NewUidTranslator(
			uidMappings,
			gidMappings,
		),
	}

	retainer := cleaner.NewRetainer()
	ovenCleaner := cleaner.NewOvenCleaner(retainer,
		cleaner.NewThreshold(int64(cmd.Graph.CleanupThresholdInMegabytes)*1024*1024),
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
	go imageRetainer.Retain(persistentImages)

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

func (cmd *ServerCommand) wireExecRunner(dadooPath, runcPath, runcRoot string, processIDGen runrunc.UidGenerator, commandRunner commandrunner.CommandRunner, shouldCleanup bool) *dadoo.ExecRunner {

	pidFileReader := &dadoo.PidFileReader{
		Clock:         clock.NewClock(),
		Timeout:       10 * time.Second,
		SleepInterval: time.Millisecond * 100,
	}

	return dadoo.NewExecRunner(
		dadooPath,
		runcPath,
		runcRoot,
		processIDGen,
		pidFileReader,
		commandRunner,
		shouldCleanup,
	)
}

func (cmd *ServerCommand) wireCgroupsStarter(logger lager.Logger) gardener.Starter {
	var cgroupsMountpoint string
	if cmd.Server.Tag != "" {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", cmd.Server.Tag))
	} else {
		cgroupsMountpoint = "/sys/fs/cgroup"
	}

	return rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), cgroupsMountpoint, commandRunner())
}

func (cmd *ServerCommand) wireExecPreparer() runrunc.ExecPreparer {
	cmdRunner := commandRunner()
	chrootMkdir := bundlerules.ChrootMkdir{
		Command:       preparerootfs.Command,
		CommandRunner: cmdRunner,
	}
	return runrunc.NewExecPreparer(&goci.BndlLoader{}, runrunc.LookupFunc(runrunc.LookupUser), runrunc.EnvFunc(runrunc.UnixEnvFor), chrootMkdir, NonRootMaxCaps, runningAsRoot)
}

func wireResolvConfigurer(depotPath string) kawasaki.DnsResolvConfigurer {
	return &kawasaki.ResolvConfigurer{
		HostsFileCompiler: &dns.HostsFileCompiler{},
		ResolvCompiler:    &dns.ResolvCompiler{},
		ResolvFilePath:    "/etc/resolv.conf",
		DepotDir:          depotPath,
	}
}

func defaultBindMounts(binInitPath string) []specs.Mount {
	return []specs.Mount{
		{Type: "sysfs", Source: "sysfs", Destination: "/sys", Options: []string{"nosuid", "noexec", "nodev", "ro"}},
		{Type: "tmpfs", Source: "tmpfs", Destination: "/dev/shm"},
		{Type: "devpts", Source: "devpts", Destination: "/dev/pts",
			Options: []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"}},
		{Type: "bind", Source: binInitPath, Destination: "/tmp/garden-init", Options: []string{"bind"}},
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
	}
}

func osSpecificBundleRules() []rundmc.BundlerRule {
	return []rundmc.BundlerRule{bundlerules.EtcMounts{Chowner: &bundlerules.OSChowner{}}}
}

package guardiancmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/nu7hatch/gouuid"
	"github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden-shed/distclient"
	quotaed_aufs "code.cloudfoundry.org/garden-shed/docker_drivers/aufs"
	"code.cloudfoundry.org/garden-shed/layercake"
	"code.cloudfoundry.org/garden-shed/layercake/cleaner"
	"code.cloudfoundry.org/garden-shed/quota_manager"
	"code.cloudfoundry.org/garden-shed/repository_fetcher"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/imageplugin"
	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/guardian/kawasaki/factory"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/kawasaki/ports"
	"code.cloudfoundry.org/guardian/kawasaki/subnets"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/guardian/metrics"
	"code.cloudfoundry.org/guardian/netplugin"
	"code.cloudfoundry.org/guardian/properties"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/dadoo"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/stopper"
	"code.cloudfoundry.org/guardian/sysinfo"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/aufs"
	"github.com/docker/docker/graph"
	_ "github.com/docker/docker/pkg/chrootarchive" // allow reexec of docker-applyLayer
	"github.com/docker/docker/pkg/reexec"
	"github.com/eapache/go-resiliency/retrier"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/localip"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"
)

// These are the maximum caps an unprivileged container process ever gets
// (it may get less if the user is not root, see NonRootMaxCaps)
var UnprivilegedMaxCaps = []string{
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_FSETID",
	"CAP_FOWNER",
	"CAP_MKNOD",
	"CAP_NET_RAW",
	"CAP_SETGID",
	"CAP_SETUID",
	"CAP_SETFCAP",
	"CAP_SETPCAP",
	"CAP_NET_BIND_SERVICE",
	"CAP_SYS_CHROOT",
	"CAP_KILL",
	"CAP_AUDIT_WRITE",
}

// These are the maximum caps a privileged container process ever gets
// (it may get less if the user is not root, see NonRootMaxCaps)
var PrivilegedMaxCaps = []string{
	"CAP_AUDIT_CONTROL",
	"CAP_AUDIT_READ",
	"CAP_AUDIT_WRITE",
	"CAP_BLOCK_SUSPEND",
	"CAP_CHOWN",
	"CAP_DAC_OVERRIDE",
	"CAP_DAC_READ_SEARCH",
	"CAP_FOWNER",
	"CAP_FSETID",
	"CAP_IPC_LOCK",
	"CAP_IPC_OWNER",
	"CAP_KILL",
	"CAP_LEASE",
	"CAP_LINUX_IMMUTABLE",
	"CAP_MAC_ADMIN",
	"CAP_MAC_OVERRIDE",
	"CAP_MKNOD",
	"CAP_NET_ADMIN",
	"CAP_NET_BIND_SERVICE",
	"CAP_NET_BROADCAST",
	"CAP_NET_RAW",
	"CAP_SETGID",
	"CAP_SETFCAP",
	"CAP_SETPCAP",
	"CAP_SETUID",
	"CAP_SYS_ADMIN",
	"CAP_SYS_BOOT",
	"CAP_SYS_CHROOT",
	"CAP_SYS_MODULE",
	"CAP_SYS_NICE",
	"CAP_SYS_PACCT",
	"CAP_SYS_PTRACE",
	"CAP_SYS_RAWIO",
	"CAP_SYS_RESOURCE",
	"CAP_SYS_TIME",
	"CAP_SYS_TTY_CONFIG",
	"CAP_SYSLOG",
	"CAP_WAKE_ALARM",
}

// These are the maximum capabilities a non-root user gets whether privileged or unprivileged
// In other words in a privileged container a non-root user still only gets the unprivileged set
// plus CAP_SYS_ADMIN.
var NonRootMaxCaps = append(UnprivilegedMaxCaps, "CAP_SYS_ADMIN")

var PrivilegedContainerNamespaces = []specs.Namespace{
	goci.NetworkNamespace, goci.PIDNamespace, goci.UTSNamespace, goci.IPCNamespace, goci.MountNamespace,
}

type GuardianCommand struct {
	Logger LagerFlag

	Server struct {
		BindIP   IPFlag `long:"bind-ip"                  description:"Bind with TCP on the given IP."`
		BindPort uint16 `long:"bind-port" default:"7777" description:"Bind with TCP on the given port."`

		BindSocket string `long:"bind-socket" default:"/tmp/garden.sock" description:"Bind with Unix on the given socket path."`

		DebugBindIP   IPFlag `long:"debug-bind-ip"                   description:"Bind the debug server on the given IP."`
		DebugBindPort uint16 `long:"debug-bind-port" default:"17013" description:"Bind the debug server to the given port."`

		Tag      string `long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`
		Rootless bool   `long:"rootless" description:"Run server in rootless mode."`
	} `group:"Server Configuration"`

	Containers struct {
		Dir            DirFlag `long:"depot" required:"true" description:"Directory in which to store container data."`
		PropertiesPath string  `long:"properties-path" description:"Path in which to store properties."`

		DefaultRootFSDir           DirFlag       `long:"default-rootfs"     description:"Default rootfs to use when not specified on container creation."`
		DefaultGraceTime           time.Duration `long:"default-grace-time" description:"Default time after which idle containers should expire."`
		DestroyContainersOnStartup bool          `long:"destroy-containers-on-startup" description:"Clean up all the existing containers on startup."`
		ApparmorProfile            string        `long:"apparmor" description:"Apparmor profile to use for unprivileged container processes"`
	} `group:"Container Lifecycle"`

	Bin struct {
		Dadoo       FileFlag `long:"dadoo-bin"     required:"true" description:"Path to the 'dadoo' binary."`
		NSTar       FileFlag `long:"nstar-bin"     required:"true" description:"Path to the 'nstar' binary."`
		Tar         FileFlag `long:"tar-bin"       required:"true" description:"Path to the 'tar' binary."`
		IPTables    FileFlag `long:"iptables-bin"  default:"/sbin/iptables" description:"path to the the iptables binary"`
		Init        FileFlag `long:"init-bin"      required:"true" description:"Path execute as pid 1 inside each container."`
		Runc        string   `long:"runc-bin"      default:"runc" description:"Path to the 'runc' binary."`
		ImagePlugin FileFlag `long:"image-plugin"           description:"Path to image plugin binary."`
	} `group:"Binary Tools"`

	Graph struct {
		Dir                         DirFlag  `long:"graph"                 description:"Directory on which to store imported rootfs graph data."`
		CleanupThresholdInMegabytes int      `long:"graph-cleanup-threshold-in-megabytes" default:"-1" description:"Disk usage of the graph dir at which cleanup should trigger, or -1 to disable graph cleanup."`
		PersistentImages            []string `long:"persistent-image" description:"Image that should never be garbage collected. Can be specified multiple times."`
	} `group:"Image Graph"`

	Docker struct {
		Registry           string   `long:"docker-registry" default:"registry-1.docker.io" description:"Docker registry API endpoint."`
		InsecureRegistries []string `long:"insecure-docker-registry" description:"Docker registry to allow connecting to even if not secure. Can be specified multiple times."`
	} `group:"Docker Image Fetching"`

	Network struct {
		Pool CIDRFlag `long:"network-pool" default:"10.254.0.0/22" description:"Network range to use for dynamically allocated container subnets."`

		AllowHostAccess bool       `long:"allow-host-access" description:"Allow network access to the host machine."`
		DenyNetworks    []CIDRFlag `long:"deny-network"      description:"Network ranges to which traffic from containers will be denied. Can be specified multiple times."`
		AllowNetworks   []CIDRFlag `long:"allow-network"     description:"Network ranges to which traffic from containers will be allowed. Can be specified multiple times."`

		DNSServers []IPFlag `long:"dns-server" description:"DNS server IP address to use instead of automatically determined servers. Can be specified multiple times."`

		ExternalIP             IPFlag `long:"external-ip"                     description:"IP address to use to reach container's mapped ports. Autodetected if not specified."`
		PortPoolStart          uint32 `long:"port-pool-start" default:"60000" description:"Start of the ephemeral port range used for mapped container ports."`
		PortPoolSize           uint32 `long:"port-pool-size"  default:"5000"  description:"Size of the port pool used for mapped container ports."`
		PortPoolPropertiesPath string `long:"port-pool-properties-path" description:"Path in which to store port pool properties."`

		Mtu int `long:"mtu" default:"1500" description:"MTU size for container network interfaces."`

		Plugin          FileFlag `long:"network-plugin"           description:"Path to network plugin binary."`
		PluginExtraArgs []string `long:"network-plugin-extra-arg" description:"Extra argument to pass to the network plugin. Can be specified multiple times."`
	} `group:"Container Networking"`

	Limits struct {
		MaxContainers uint64 `long:"max-containers" default:"0" description:"Maximum number of containers that can be created."`
	} `group:"Limits"`

	Metrics struct {
		EmissionInterval time.Duration `long:"metrics-emission-interval" default:"1m" description:"Interval on which to emit metrics."`

		DropsondeOrigin      string `long:"dropsonde-origin"      default:"garden-linux"   description:"Origin identifier for Dropsonde-emitted metrics."`
		DropsondeDestination string `long:"dropsonde-destination" default:"127.0.0.1:3457" description:"Destination for Dropsonde-emitted metrics."`
	} `group:"Metrics"`
}

var idMappings rootfs_provider.MappingList

func init() {
	if reexec.Init() {
		os.Exit(0)
	}

	maxId := uint32(sysinfo.Min(sysinfo.MustGetMaxValidUID(), sysinfo.MustGetMaxValidGID()))
	idMappings = rootfs_provider.MappingList{
		{
			ContainerID: 0,
			HostID:      maxId,
			Size:        1,
		},
		{
			ContainerID: 1,
			HostID:      1,
			Size:        maxId - 1,
		},
	}
}

func (cmd *GuardianCommand) Execute([]string) error {
	return <-ifrit.Invoke(sigmon.New(cmd)).Wait()
}

func (cmd *GuardianCommand) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger, reconfigurableSink := cmd.Logger.Logger("guardian")

	if cmd.Server.Rootless {
		logger.Info("rootless-mode-on")
	}

	if err := exec.Command("modprobe", "aufs").Run(); err != nil {
		logger.Error("unable-to-load-aufs", err)
	}

	propManager, err := cmd.loadProperties(logger, cmd.Containers.PropertiesPath)
	if err != nil {
		return err
	}

	portPoolState, err := ports.LoadState(cmd.Network.PortPoolPropertiesPath)
	if err != nil {
		logger.Error("failed-to-parse-port-pool-properties", err)
	}

	portPool, err := ports.NewPool(
		cmd.Network.PortPoolStart,
		cmd.Network.PortPoolSize,
		portPoolState,
	)
	if err != nil {
		return fmt.Errorf("invalid pool range: %s", err)
	}

	networker, iptablesStarter, err := cmd.wireNetworker(logger, propManager, portPool)
	if err != nil {
		logger.Error("failed-to-wire-networker", err)
		return err
	}

	restorer := gardener.NewRestorer(networker)
	if cmd.Containers.DestroyContainersOnStartup {
		restorer = &gardener.NoopRestorer{}
	}

	var volumeCreator gardener.VolumeCreator = nil
	if !cmd.Server.Rootless {
		volumeCreator = cmd.wireVolumeCreator(logger, cmd.Graph.Dir.Path(), cmd.Docker.InsecureRegistries, cmd.Graph.PersistentImages)
	}

	starters := []gardener.Starter{}
	if !cmd.Server.Rootless {
		starters = []gardener.Starter{cmd.wireRunDMCStarter(logger), iptablesStarter}
	}

	backend := &gardener.Gardener{
		UidGenerator:    cmd.wireUidGenerator(),
		Starters:        starters,
		SysInfoProvider: sysinfo.NewProvider(cmd.Containers.Dir.Path()),
		Networker:       networker,
		VolumeCreator:   volumeCreator,
		Containerizer:   cmd.wireContainerizer(logger, cmd.Containers.Dir.Path(), cmd.Bin.Dadoo.Path(), cmd.Bin.Runc, cmd.Bin.NSTar.Path(), cmd.Bin.Tar.Path(), cmd.Containers.DefaultRootFSDir.Path(), cmd.Containers.ApparmorProfile, propManager),
		PropertyManager: propManager,
		MaxContainers:   cmd.Limits.MaxContainers,
		Restorer:        restorer,

		Logger: logger,
	}

	var listenNetwork, listenAddr string
	if cmd.Server.BindIP != nil {
		listenNetwork = "tcp"
		listenAddr = fmt.Sprintf("%s:%d", cmd.Server.BindIP.IP(), cmd.Server.BindPort)
	} else {
		listenNetwork = "unix"
		listenAddr = cmd.Server.BindSocket
	}

	gardenServer := server.New(listenNetwork, listenAddr, cmd.Containers.DefaultGraceTime, backend, logger.Session("api"))

	cmd.initializeDropsonde(logger)

	metricsProvider := cmd.wireMetricsProvider(logger, cmd.Containers.Dir.Path(), cmd.Graph.Dir.Path())

	metronNotifier := cmd.wireMetronNotifier(logger, metricsProvider)
	metronNotifier.Start()

	if cmd.Server.DebugBindIP != nil {
		addr := fmt.Sprintf("%s:%d", cmd.Server.DebugBindIP.IP(), cmd.Server.DebugBindPort)
		metrics.StartDebugServer(addr, reconfigurableSink, metricsProvider)
	}

	err = gardenServer.Start()
	if err != nil {
		logger.Error("failed-to-start-server", err)
		return err
	}

	close(ready)

	logger.Info("started", lager.Data{
		"network": listenNetwork,
		"addr":    listenAddr,
	})

	<-signals

	gardenServer.Stop()

	cmd.saveProperties(logger, cmd.Containers.PropertiesPath, propManager)

	portPoolState = portPool.RefreshState()
	ports.SaveState(cmd.Network.PortPoolPropertiesPath, portPoolState)

	return nil
}

func (cmd *GuardianCommand) loadProperties(logger lager.Logger, propertiesPath string) (*properties.Manager, error) {
	propManager, err := properties.Load(propertiesPath)
	if err != nil {
		logger.Error("failed-to-load-properties", err, lager.Data{"propertiesPath": propertiesPath})
		return &properties.Manager{}, err
	}

	return propManager, nil
}

func (cmd *GuardianCommand) saveProperties(logger lager.Logger, propertiesPath string, propManager *properties.Manager) {
	if propertiesPath != "" {
		err := properties.Save(propertiesPath, propManager)
		if err != nil {
			logger.Error("failed-to-save-properties", err, lager.Data{"propertiesPath": propertiesPath})
		}
	}
}

func (cmd *GuardianCommand) wireUidGenerator() gardener.UidGeneratorFunc {
	return gardener.UidGeneratorFunc(func() string { return mustStringify(uuid.NewV4()) })
}

func (cmd *GuardianCommand) wireRunDMCStarter(logger lager.Logger) gardener.Starter {
	var cgroupsMountpoint string
	if cmd.Server.Tag != "" {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", cmd.Server.Tag))
	} else {
		cgroupsMountpoint = "/sys/fs/cgroup"
	}

	return rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), cgroupsMountpoint, linux_command_runner.New())
}

func (cmd *GuardianCommand) wireNetworker(log lager.Logger, propManager kawasaki.ConfigStore, portPool *ports.PortPool) (gardener.Networker, gardener.Starter, error) {
	externalIP, err := defaultExternalIP(cmd.Network.ExternalIP)
	if err != nil {
		return nil, nil, err
	}

	dnsServers := make([]net.IP, len(cmd.Network.DNSServers))
	for i, ip := range cmd.Network.DNSServers {
		dnsServers[i] = ip.IP()
	}

	if cmd.Network.Plugin.Path() != "" {
		resolvConfigurer := &kawasaki.ResolvConfigurer{
			HostsFileCompiler:  &dns.HostsFileCompiler{},
			ResolvFileCompiler: &dns.ResolvFileCompiler{},
			FileWriter:         &dns.RootfsWriter{},
			IDMapReader:        &kawasaki.RootIdMapReader{},
		}
		externalNetworker := netplugin.New(
			linux_command_runner.New(),
			propManager,
			externalIP,
			dnsServers,
			resolvConfigurer,
			cmd.Network.Plugin.Path(),
			cmd.Network.PluginExtraArgs,
		)
		return externalNetworker, externalNetworker, nil
	}

	var denyNetworksList []string
	for _, network := range cmd.Network.DenyNetworks {
		denyNetworksList = append(denyNetworksList, network.String())
	}

	interfacePrefix := fmt.Sprintf("w%s", cmd.Server.Tag)
	chainPrefix := fmt.Sprintf("w-%s-", cmd.Server.Tag)
	idGenerator := kawasaki.NewSequentialIDGenerator(time.Now().UnixNano())
	iptRunner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: log.Session("iptables-runner")}
	ipTables := iptables.New(cmd.Bin.IPTables.Path(), iptRunner, chainPrefix)
	ipTablesStarter := iptables.NewStarter(ipTables, cmd.Network.AllowHostAccess, interfacePrefix, denyNetworksList, cmd.Containers.DestroyContainersOnStartup)

	networker := kawasaki.New(
		cmd.Bin.IPTables.Path(),
		kawasaki.SpecParserFunc(kawasaki.ParseSpec),
		subnets.NewPool(cmd.Network.Pool.CIDR()),
		kawasaki.NewConfigCreator(idGenerator, interfacePrefix, chainPrefix, externalIP, dnsServers, cmd.Network.Mtu),
		propManager,
		factory.NewDefaultConfigurer(ipTables),
		portPool,
		iptables.NewPortForwarder(ipTables),
		iptables.NewFirewallOpener(ipTables),
	)

	return networker, ipTablesStarter, nil
}

func (cmd *GuardianCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	if graphRoot == "" {
		return gardener.NoopVolumeCreator{}
	}

	if cmd.Bin.ImagePlugin.Path() != "" {
		return imageplugin.New(cmd.Bin.ImagePlugin.Path(), linux_command_runner.New())
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
				DefaultRootFSPath: cmd.Containers.DefaultRootFSDir.Path(),
				IDProvider:        repository_fetcher.LayerIDProvider{},
			},
			RemoteFetcher: repository_fetcher.NewRemote(
				logger,
				cmd.Docker.Registry,
				cake,
				distclient.NewDialer(insecureRegistries),
				repository_fetcher.VerifyFunc(repository_fetcher.Verify),
			),
		},
		Logger: logger,
	}

	rootFSNamespacer := &rootfs_provider.UidNamespacer{
		Translator: rootfs_provider.NewUidTranslator(
			idMappings, // uid
			idMappings, // gid
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

func (cmd *GuardianCommand) wireContainerizer(log lager.Logger, depotPath, dadooPath, runcPath, nstarPath, tarPath, defaultRootFSPath, appArmorProfile string, properties gardener.PropertyManager) *rundmc.Containerizer {
	depot := depot.New(depotPath)

	commandRunner := linux_command_runner.New()
	chrootMkdir := bundlerules.ChrootMkdir{
		Command:       preparerootfs.Command,
		CommandRunner: commandRunner,
	}

	pidFileReader := &dadoo.PidFileReader{
		Clock:         clock.NewClock(),
		Timeout:       10 * time.Second,
		SleepInterval: time.Millisecond * 100,
	}

	runcrunner := runrunc.New(
		commandRunner,
		runrunc.NewLogRunner(commandRunner, runrunc.LogDir(os.TempDir()).GenerateLogFile),
		goci.RuncBinary(runcPath),
		dadooPath,
		runcPath,
		runrunc.NewExecPreparer(&goci.BndlLoader{}, runrunc.LookupFunc(runrunc.LookupUser), chrootMkdir, NonRootMaxCaps),
		dadoo.NewExecRunner(
			dadooPath,
			runcPath,
			cmd.wireUidGenerator(),
			pidFileReader,
			linux_command_runner.New()),
	)

	mounts := []specs.Mount{
		{Type: "sysfs", Source: "sysfs", Destination: "/sys", Options: []string{"nosuid", "noexec", "nodev", "ro"}},
		{Type: "tmpfs", Source: "tmpfs", Destination: "/dev/shm"},
		{Type: "devpts", Source: "devpts", Destination: "/dev/pts",
			Options: []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"}},
		{Type: "bind", Source: cmd.Bin.Init.Path(), Destination: "/tmp/garden-init", Options: []string{"bind"}},
	}

	privilegedMounts := append(mounts,
		specs.Mount{Type: "proc", Source: "proc", Destination: "/proc", Options: []string{"nosuid", "noexec", "nodev"}},
	)

	unprivilegedMounts := append(mounts,
		specs.Mount{Type: "proc", Source: "proc", Destination: "/proc", Options: []string{"nosuid", "noexec", "nodev"}},
	)

	rwm := "rwm"
	character := "c"
	var majorMinor = func(i int64) *int64 {
		return &i
	}

	var worldReadWrite os.FileMode = 0666
	fuseDevice := specs.Device{
		Path:     "/dev/fuse",
		Type:     "c",
		Major:    10,
		Minor:    229,
		FileMode: &worldReadWrite,
	}

	denyAll := specs.DeviceCgroup{Allow: false, Access: &rwm}
	allowedDevices := []specs.DeviceCgroup{
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(3), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(5), Minor: majorMinor(0), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(8), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(9), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(5), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(7), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(7), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(fuseDevice.Major), Minor: majorMinor(fuseDevice.Minor), Allow: true},
	}

	baseProcess := specs.Process{
		Capabilities: UnprivilegedMaxCaps,
		Args:         []string{"/tmp/garden-init"},
		Cwd:          "/",
	}

	baseBundle := goci.Bundle().
		WithNamespaces(PrivilegedContainerNamespaces...).
		WithResources(&specs.Resources{Devices: append([]specs.DeviceCgroup{denyAll}, allowedDevices...)}).
		WithRootFS(defaultRootFSPath).
		WithDevices(fuseDevice).
		WithProcess(baseProcess)

	unprivilegedBundle := baseBundle.
		WithNamespace(goci.UserNamespace).
		WithUIDMappings(idMappings...).
		WithGIDMappings(idMappings...).
		WithMounts(unprivilegedMounts...).
		WithMaskedPaths(defaultMaskedPaths())

	unprivilegedBundle.Spec.Linux.Seccomp = seccomp
	if appArmorProfile != "" {
		unprivilegedBundle.Spec.Process.ApparmorProfile = appArmorProfile
	}

	privilegedBundle := baseBundle.
		WithMounts(privilegedMounts...).
		WithCapabilities(PrivilegedMaxCaps...)

	template := &rundmc.BundleTemplate{
		Rules: []rundmc.BundlerRule{
			bundlerules.Base{
				PrivilegedBase:   privilegedBundle,
				UnprivilegedBase: unprivilegedBundle,
			},
			bundlerules.RootFS{
				ContainerRootUID: idMappings.Map(0),
				ContainerRootGID: idMappings.Map(0),
				MkdirChown:       chrootMkdir,
			},
			bundlerules.Limits{},
			bundlerules.BindMounts{},
			bundlerules.Env{},
			bundlerules.Hostname{},
		},
	}

	log.Info("base-bundles", lager.Data{
		"privileged":   privilegedBundle,
		"unprivileged": unprivilegedBundle,
	})

	eventStore := rundmc.NewEventStore(properties)
	stateStore := rundmc.NewStateStore(properties)

	nstar := rundmc.NewNstarRunner(nstarPath, tarPath, linux_command_runner.New())
	stopper := stopper.New(stopper.NewRuncStateCgroupPathResolver("/run/runc"), nil, retrier.New(retrier.ConstantBackoff(10, 1*time.Second), nil))
	return rundmc.New(depot, template, runcrunner, &goci.BndlLoader{}, nstar, stopper, eventStore, stateStore)
}

func (cmd *GuardianCommand) wireMetricsProvider(log lager.Logger, depotPath, graphRoot string) metrics.Metrics {
	var backingStoresPath string
	if graphRoot != "" {
		backingStoresPath = filepath.Join(graphRoot, "backing_stores")
	}

	return metrics.NewMetrics(log, backingStoresPath, depotPath)
}

func (cmd *GuardianCommand) wireMetronNotifier(log lager.Logger, metricsProvider metrics.Metrics) *metrics.PeriodicMetronNotifier {
	return metrics.NewPeriodicMetronNotifier(
		log, metricsProvider, cmd.Metrics.EmissionInterval, clock.NewClock(),
	)
}

func (cmd *GuardianCommand) initializeDropsonde(log lager.Logger) {
	err := dropsonde.Initialize(cmd.Metrics.DropsondeDestination, cmd.Metrics.DropsondeOrigin)
	if err != nil {
		log.Error("failed to initialize dropsonde", err)
	}
}

func defaultExternalIP(ip IPFlag) (net.IP, error) {
	if ip != nil {
		return ip.IP(), nil
	}

	localIP, err := localip.LocalIP()
	if err != nil {
		return nil, fmt.Errorf("Couldn't determine local IP to use for --external-ip parameter. You can use the --external-ip flag to pass an external IP explicitly.")
	}

	return net.ParseIP(localIP), nil
}

func defaultMaskedPaths() []string {
	return []string{
		"/proc/kcore",
		"/proc/latency_stats",
		"/proc/timer_stats",
		"/proc/sched_debug",
	}
}

func mustStringify(s interface{}, e error) string {
	if e != nil {
		panic(e)
	}

	return fmt.Sprintf("%s", s)
}

func mustOpen(path string) io.ReadCloser {
	if r, err := os.Open(path); err != nil {
		panic(err)
	} else {
		return r
	}
}

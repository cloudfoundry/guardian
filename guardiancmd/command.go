package guardiancmd

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/cloudfoundry-incubator/garden-shed/distclient"
	"github.com/nu7hatch/gouuid"
	"github.com/opencontainers/specs/specs-go"

	quotaed_aufs "github.com/cloudfoundry-incubator/garden-shed/docker_drivers/aufs"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/cloudfoundry-incubator/garden-shed/layercake/cleaner"
	"github.com/cloudfoundry-incubator/garden-shed/quota_manager"
	"github.com/cloudfoundry-incubator/garden-shed/repository_fetcher"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/factory"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/ports"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/cloudfoundry-incubator/guardian/logging"
	"github.com/cloudfoundry-incubator/guardian/metrics"
	"github.com/cloudfoundry-incubator/guardian/netplugin"
	"github.com/cloudfoundry-incubator/guardian/properties"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/cloudfoundry-incubator/guardian/rundmc/preparerootfs"
	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/sysinfo"
	"github.com/cloudfoundry/dropsonde"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/aufs"
	"github.com/docker/docker/graph"
	_ "github.com/docker/docker/pkg/chrootarchive" // allow reexec of docker-applyLayer
	"github.com/docker/docker/pkg/reexec"
	"github.com/eapache/go-resiliency/retrier"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/localip"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"
)

const OciStateDir = "/var/run/opencontainer/containers"

var DefaultCapabilities = []string{
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

		Tag string `long:"tag" description:"Optional 2-character identifier used for namespacing global configuration."`
	} `group:"Server Configuration"`

	Containers struct {
		Dir DirFlag `long:"depot" required:"true" description:"Directory in which to store container data."`

		DefaultRootFSDir DirFlag       `long:"default-rootfs"     description:"Default rootfs to use when not specified on container creation."`
		DefaultGraceTime time.Duration `long:"default-grace-time" description:"Default time after which idle containers should expire."`
	} `group:"Container Lifecycle"`

	Bin struct {
		IODaemon FileFlag `long:"iodaemon-bin" required:"true" description:"Path to the 'iodaemon' binary."`
		Dadoo    FileFlag `long:"dadoo-bin" required:"true" description:"Path to the 'dadoo' binary."`
		NSTar    FileFlag `long:"nstar-bin"    required:"true" description:"Path to the 'nstar' binary."`
		Tar      FileFlag `long:"tar-bin"      required:"true" description:"Path to the 'tar' binary."`
		Kawasaki FileFlag `long:"kawasaki-bin" required:"true" description:"Path to the 'kawasaki' network hook binary."`
		Init     FileFlag `long:"init-bin"     required:"true" description:"Path execute as pid 1 inside each container."`
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

		ExternalIP    IPFlag `long:"external-ip"                     description:"IP address to use to reach container's mapped ports. Autodetected if not specified."`
		PortPoolStart uint32 `long:"port-pool-start" default:"60000" description:"Start of the ephemeral port range used for mapped container ports."`
		PortPoolSize  uint32 `long:"port-pool-size"  default:"5000"  description:"Size of the port pool used for mapped container ports."`

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
	if reexec.Init() {
		return nil
	}

	return <-ifrit.Invoke(sigmon.New(cmd)).Wait()
}

func (cmd *GuardianCommand) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger, reconfigurableSink := cmd.Logger.Logger("guardian")

	var denyNetworksList []string
	for _, network := range cmd.Network.DenyNetworks {
		denyNetworksList = append(denyNetworksList, network.String())
	}

	externalIPAddr, err := defaultExternalIP(cmd.Network.ExternalIP)
	if err != nil {
		return err
	}

	interfacePrefix := fmt.Sprintf("w%s", cmd.Server.Tag)
	chainPrefix := fmt.Sprintf("w-%s-", cmd.Server.Tag)
	ipt := cmd.wireIptables(logger, chainPrefix)

	propManager := properties.NewManager()

	var networker gardener.Networker = netplugin.New(cmd.Network.Plugin.Path(), cmd.Network.PluginExtraArgs...)
	if cmd.Network.Plugin == "" {
		dnsIPs := make([]net.IP, len(cmd.Network.DNSServers))
		for i, ip := range cmd.Network.DNSServers {
			dnsIPs[i] = ip.IP()
		}

		networker = cmd.wireNetworker(logger, cmd.Bin.Kawasaki.Path(), cmd.Server.Tag, cmd.Network.Pool.CIDR(), externalIPAddr, dnsIPs, ipt, interfacePrefix, chainPrefix, propManager)
	}

	backend := &gardener.Gardener{
		UidGenerator:    cmd.wireUidGenerator(),
		Starter:         cmd.wireStarter(logger, ipt, cmd.Network.AllowHostAccess, interfacePrefix, denyNetworksList),
		SysInfoProvider: sysinfo.NewProvider(cmd.Containers.Dir.Path()),
		Networker:       networker,
		VolumeCreator:   cmd.wireVolumeCreator(logger, cmd.Graph.Dir.Path(), cmd.Docker.InsecureRegistries, cmd.Graph.PersistentImages),
		Containerizer:   cmd.wireContainerizer(logger, cmd.Containers.Dir.Path(), cmd.Bin.IODaemon.Path(), cmd.Bin.Dadoo.Path(), cmd.Bin.NSTar.Path(), cmd.Bin.Tar.Path(), cmd.Containers.DefaultRootFSDir.Path(), propManager),
		PropertyManager: propManager,
		MaxContainers:   cmd.Limits.MaxContainers,

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

	return nil
}

func (cmd *GuardianCommand) wireUidGenerator() gardener.UidGeneratorFunc {
	return gardener.UidGeneratorFunc(func() string { return mustStringify(uuid.NewV4()) })
}

func (cmd *GuardianCommand) wireStarter(logger lager.Logger, ipt *iptables.IPTables, allowHostAccess bool, nicPrefix string, denyNetworks []string) gardener.Starter {
	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger.Session("runner")}

	return &StartAll{starters: []gardener.Starter{
		rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), mustOpen("/proc/self/cgroup"), path.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", cmd.Server.Tag)), runner),
		iptables.NewStarter(ipt, allowHostAccess, nicPrefix, denyNetworks),
	}}
}

func (cmd *GuardianCommand) wireIptables(logger lager.Logger, prefix string) *iptables.IPTables {
	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger.Session("iptables-runner")}
	return iptables.New(runner, prefix)
}

func (cmd *GuardianCommand) wireNetworker(
	log lager.Logger,
	kawasakiBin string,
	tag string,
	networkPoolCIDR *net.IPNet,
	externalIP net.IP,
	dnsServers []net.IP,
	ipt *iptables.IPTables,
	interfacePrefix string,
	chainPrefix string,
	propManager *properties.Manager,
) gardener.Networker {
	idGenerator := kawasaki.NewSequentialIDGenerator(time.Now().UnixNano())
	portPool, err := ports.NewPool(cmd.Network.PortPoolStart, cmd.Network.PortPoolSize, ports.State{})
	if err != nil {
		log.Fatal("invalid pool range", err)
	}

	return kawasaki.New(
		kawasakiBin,
		kawasaki.SpecParserFunc(kawasaki.ParseSpec),
		subnets.NewPool(networkPoolCIDR),
		kawasaki.NewConfigCreator(idGenerator, interfacePrefix, chainPrefix, externalIP, dnsServers),
		factory.NewDefaultConfigurer(ipt),
		propManager,
		portPool,
		iptables.NewPortForwarder(ipt),
		iptables.NewFirewallOpener(ipt),
	)
}

func (cmd *GuardianCommand) wireVolumeCreator(logger lager.Logger, graphRoot string, insecureRegistries, persistentImages []string) gardener.VolumeCreator {
	if graphRoot == "" {
		return gardener.NoopVolumeCreator{}
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

	repoFetcher := &repository_fetcher.CompositeFetcher{
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
	}

	rootFSNamespacer := &rootfs_provider.UidNamespacer{
		Logger: logger,
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

func (cmd *GuardianCommand) wireContainerizer(log lager.Logger, depotPath, iodaemonPath, dadooPath, nstarPath, tarPath, defaultRootFSPath string, properties gardener.PropertyManager) *rundmc.Containerizer {
	depot := depot.New(depotPath)

	commandRunner := linux_command_runner.New()
	chrootMkdir := bundlerules.ChrootMkdir{
		Command:       preparerootfs.Command,
		CommandRunner: commandRunner,
	}

	pidFileReader := &process_tracker.PidFileReader{
		Clock:         clock.NewClock(),
		Timeout:       10 * time.Second,
		SleepInterval: time.Millisecond * 100,
	}

	runcrunner := runrunc.New(
		commandRunner,
		runrunc.NewLogRunner(commandRunner, runrunc.LogDir(os.TempDir()).GenerateLogFile),
		goci.RuncBinary("runc"),
		dadooPath,
		runrunc.NewExecPreparer(&goci.BndlLoader{}, runrunc.LookupFunc(runrunc.LookupUser), chrootMkdir),
		runrunc.NewExecRunner(cmd.wireUidGenerator(), goci.RuncBinary("runc"),
			process_tracker.New(path.Join(os.TempDir(), fmt.Sprintf("garden-%s", cmd.Server.Tag), "processes"), iodaemonPath, commandRunner, pidFileReader),
			&runrunc.ProcessJsonCleaner{}),
	)

	mounts := []specs.Mount{
		specs.Mount{Type: "proc", Source: "proc", Destination: "/proc"},
		specs.Mount{Type: "tmpfs", Source: "tmpfs", Destination: "/dev/shm"},
		specs.Mount{Type: "devpts", Source: "devpts", Destination: "/dev/pts",
			Options: []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620"}},
		specs.Mount{Type: "bind", Source: cmd.Bin.Init.Path(), Destination: "/tmp/garden-init", Options: []string{"bind"}},
	}

	rwm := "rwm"
	character := "c"
	var majorMinor = func(i int64) *int64 {
		return &i
	}

	denyAll := specs.DeviceCgroup{Allow: false, Access: &rwm}
	allowedDevices := []specs.DeviceCgroup{
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(3), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(5), Minor: majorMinor(0), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(8), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(9), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(5), Allow: true},
		{Access: &rwm, Type: &character, Major: majorMinor(1), Minor: majorMinor(7), Allow: true},
	}

	baseProcess := specs.Process{
		Capabilities: DefaultCapabilities,
		Args:         []string{"/tmp/garden-init"},
		Cwd:          "/",
	}

	baseBundle := goci.Bundle().
		WithNamespaces(PrivilegedContainerNamespaces...).
		WithResources(&specs.Resources{Devices: append([]specs.DeviceCgroup{denyAll}, allowedDevices...)}).
		WithMounts(mounts...).
		WithRootFS(defaultRootFSPath).
		WithProcess(baseProcess)

	unprivilegedBundle := baseBundle.
		WithNamespace(goci.UserNamespace).
		WithUIDMappings(idMappings...).
		WithGIDMappings(idMappings...)

	template := &rundmc.BundleTemplate{
		Rules: []rundmc.BundlerRule{
			bundlerules.Base{
				PrivilegedBase:   baseBundle,
				UnprivilegedBase: unprivilegedBundle,
			},
			bundlerules.RootFS{
				ContainerRootUID: idMappings.Map(0),
				ContainerRootGID: idMappings.Map(0),
				MkdirChown:       chrootMkdir,
			},
			bundlerules.Limits{},
			bundlerules.Hooks{LogFilePattern: filepath.Join(depotPath, "%s", "network.log")},
			bundlerules.BindMounts{},
			bundlerules.Env{},
			bundlerules.PrivilegedCaps{},
		},
	}

	log.Info("base-bundles", lager.Data{
		"privileged":   baseBundle,
		"unprivileged": unprivilegedBundle,
	})

	eventStore := rundmc.NewEventStore(properties)
	nstar := rundmc.NewNstarRunner(nstarPath, tarPath, linux_command_runner.New())

	return rundmc.New(depot, template, runcrunner, &goci.BndlLoader{}, nstar, eventStore)
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

type StartAll struct {
	starters []gardener.Starter
}

func (s *StartAll) Start() error {
	for _, starter := range s.starters {
		if err := starter.Start(); err != nil {
			return err
		}
	}

	return nil
}

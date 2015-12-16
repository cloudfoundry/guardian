package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/cf-debug-server"
	"github.com/cloudfoundry-incubator/cf-lager"
	quotaed_aufs "github.com/cloudfoundry-incubator/garden-shed/docker_drivers/aufs"
	"github.com/cloudfoundry-incubator/garden-shed/layercake"
	"github.com/cloudfoundry-incubator/garden-shed/pkg/retrier"
	"github.com/cloudfoundry-incubator/garden-shed/repository_fetcher"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/configure"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/devices"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/netns"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/ports"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/cloudfoundry-incubator/guardian/logging"
	"github.com/cloudfoundry-incubator/guardian/properties"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/depot"
	"github.com/cloudfoundry-incubator/guardian/rundmc/process_tracker"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/sysinfo"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/aufs"
	"github.com/docker/docker/graph"
	_ "github.com/docker/docker/pkg/chrootarchive" // allow reexec of docker-applyLayer
	"github.com/docker/docker/pkg/reexec"
	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/localip"
	"github.com/rosenhouse/ducati"
)

var PrivilegedContainerNamespaces = []specs.Namespace{
	goci.NetworkNamespace, goci.PIDNamespace, goci.UTSNamespace, goci.IPCNamespace, goci.MountNamespace,
}

var listenNetwork = flag.String(
	"listenNetwork",
	"unix",
	"how to listen on the address (unix, tcp, etc.)",
)

var listenAddr = flag.String(
	"listenAddr",
	"/tmp/garden.sock",
	"address to listen on",
)

var snapshotsPath = flag.String(
	"snapshots",
	"",
	"directory in which to store container state to persist through restarts",
)

var binPath = flag.String(
	"bin",
	"",
	"directory containing backend-specific scripts (i.e. ./create.sh)",
)

var iodaemonBin = flag.String(
	"iodaemonBin",
	"",
	"path to iodaemon binary",
)

var depotPath = flag.String(
	"depot",
	"",
	"directory in which to store containers",
)

var rootFSPath = flag.String(
	"rootfs",
	"",
	"directory of the rootfs for the containers",
)

var disableQuotas = flag.Bool(
	"disableQuotas",
	false,
	"disable disk quotas",
)

var graceTime = flag.Duration(
	"containerGraceTime",
	0,
	"time after which to destroy idle containers",
)

var portPoolStart = flag.Uint(
	"portPoolStart",
	60000,
	"start of ephemeral port range used for mapped container ports",
)

var portPoolSize = flag.Uint(
	"portPoolSize",
	5000,
	"size of port pool used for mapped container ports",
)

var networkPool = flag.String("networkPool",
	"10.254.0.0/22",
	"Pool of dynamically allocated container subnets")

var denyNetworks = flag.String(
	"denyNetworks",
	"",
	"CIDR blocks representing IPs to blacklist",
)

var allowNetworks = flag.String(
	"allowNetworks",
	"",
	"CIDR blocks representing IPs to whitelist",
)

var graphRoot = flag.String(
	"graph",
	"/var/lib/garden-docker-graph",
	"docker image graph",
)

var dockerRegistry = flag.String(
	"registry",
	"",
	///registry.IndexServerAddress(),
	"docker registry API endpoint",
)

var insecureRegistries = flag.String(
	"insecureDockerRegistryList",
	"",
	"comma-separated list of docker registries to allow connection to even if they are not secure",
)

var tag = flag.String(
	"tag",
	"",
	"server-wide identifier used for 'global' configuration, must be less than 3 character long",
)

var dropsondeOrigin = flag.String(
	"dropsondeOrigin",
	"garden-linux",
	"Origin identifier for dropsonde-emitted metrics.",
)

var dropsondeDestination = flag.String(
	"dropsondeDestination",
	"localhost:3457",
	"Destination for dropsonde-emitted metrics.",
)

var allowHostAccess = flag.Bool(
	"allowHostAccess",
	false,
	"allow network access to host",
)

var iptablesLogMethod = flag.String(
	"iptablesLogMethod",
	"kernel",
	"type of iptable logging to use, one of 'kernel' or 'nflog' (default: kernel)",
)

var mtu = flag.Int(
	"mtu",
	1500,
	"MTU size for container network interfaces")

var externalIP = flag.String(
	"externalIP",
	"",
	"IP address to use to reach container's mapped ports")

var maxContainers = flag.Uint(
	"maxContainers",
	0,
	"Maximum number of containers that can be created")

var networkModule = flag.String(
	"networkModule",
	"kawasaki",
	"Name of network module")

func main() {
	if reexec.Init() {
		return
	}

	cf_debug_server.AddFlags(flag.CommandLine)
	cf_lager.AddFlags(flag.CommandLine)
	flag.Parse()

	logger, _ := cf_lager.New("guardian")

	if *depotPath == "" {
		missing("-depot")
	}

	if *iodaemonBin == "" {
		missing("-iodaemonBin")
	}

	resolvedRootFSPath, err := filepath.EvalSymlinks(*rootFSPath)
	if err != nil {
		panic(err)
	}

	_, networkPoolCIDR, err := net.ParseCIDR(*networkPool)
	if err != nil {
		panic(err)
	}

	interfacePrefix := fmt.Sprintf("w%s", *tag)
	chainPrefix := fmt.Sprintf("w-%s-instance", *tag)
	iptablesMgr := wireIptables(logger, *tag, *allowHostAccess, interfacePrefix, chainPrefix)
	externalIPAddr, err := parseExternalIP(*externalIP)
	if err != nil {
		panic(err)
	}

	sysInfoProvider := sysinfo.NewProvider(*depotPath)

	propManager := properties.NewManager()

	backend := &gardener.Gardener{
		SysInfoProvider: sysInfoProvider,
		UidGenerator:    wireUidGenerator(),
		Starter:         wireStarter(logger, iptablesMgr),
		Networker:       wireNetworker(logger, *tag, networkPoolCIDR, externalIPAddr, iptablesMgr, interfacePrefix, chainPrefix, propManager, *networkModule),
		VolumeCreator:   wireVolumeCreator(logger, *graphRoot),
		Containerizer:   wireContainerizer(logger, *depotPath, *iodaemonBin, resolvedRootFSPath),
		Logger:          logger,
		PropertyManager: propManager,
	}

	gardenServer := server.New(*listenNetwork, *listenAddr, *graceTime, backend, logger.Session("api"))

	err = gardenServer.Start()
	if err != nil {
		logger.Fatal("failed-to-start-server", err)
	}

	signals := make(chan os.Signal, 1)

	go func() {
		<-signals
		gardenServer.Stop()
		os.Exit(0)
	}()

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	logger.Info("started", lager.Data{
		"network": *listenNetwork,
		"addr":    *listenAddr,
	})

	select {}
}

func wireUidGenerator() gardener.UidGeneratorFunc {
	return gardener.UidGeneratorFunc(func() string { return mustStringify(uuid.NewV4()) })
}

func wireStarter(logger lager.Logger, iptablesMgr *iptables.Manager) gardener.Starter {
	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger.Session("runner")}

	return &StartAll{starters: []gardener.Starter{
		rundmc.NewStarter(logger, mustOpen("/proc/cgroups"), path.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", *tag)), runner),
		iptablesMgr,
	}}
}

func wireIptables(logger lager.Logger, tag string, allowHostAccess bool, interfacePrefix, chainPrefix string) *iptables.Manager {
	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: logger.Session("iptables-runner")}

	filterConfig := iptables.FilterConfig{
		AllowHostAccess: allowHostAccess,
		InputChain:      fmt.Sprintf("w-%s-input", tag),
		ForwardChain:    fmt.Sprintf("w-%s-forward", tag),
		DefaultChain:    fmt.Sprintf("w-%s-default", tag),
	}

	natConfig := iptables.NATConfig{
		PreroutingChain:  fmt.Sprintf("w-%s-prerouting", tag),
		PostroutingChain: fmt.Sprintf("w-%s-postrouting", tag),
	}

	return iptables.NewManager(
		filterConfig,
		natConfig,
		interfacePrefix,
		chainPrefix,
		runner,
		logger,
	)
}

func wireNetworker(log lager.Logger, tag string, networkPoolCIDR *net.IPNet, externalIP net.IP, iptablesMgr kawasaki.IPTablesConfigurer, interfacePrefix, chainPrefix string, propManager *properties.Manager, networkModule string) gardener.Networker {
	runner := &logging.Runner{CommandRunner: linux_command_runner.New(), Logger: log.Session("network-runner")}

	hostConfigurer := &configure.Host{
		Veth:   &devices.VethCreator{},
		Link:   &devices.Link{Name: "guardian"},
		Bridge: &devices.Bridge{},
		Logger: log.Session("network-host-configurer"),
	}

	containerCfgApplier := &configure.Container{
		Logger: log.Session("network-container-configurer"),
		Link:   &devices.Link{Name: "guardian"},
	}

	idGenerator := kawasaki.NewSequentialIDGenerator(time.Now().UnixNano())
	portPool, err := ports.NewPool(uint32(*portPoolStart), uint32(*portPoolSize), ports.State{})
	if err != nil {
		log.Fatal("invalid pool range", err)
	}

	switch networkModule {
	case "kawasaki":
		return kawasaki.New(
			kawasaki.NewManager(runner, "/var/run/netns"),
			kawasaki.SpecParserFunc(kawasaki.ParseSpec),
			subnets.NewPool(networkPoolCIDR),
			kawasaki.NewConfigCreator(idGenerator, interfacePrefix, chainPrefix, externalIP),
			kawasaki.NewConfigurer(
				hostConfigurer,
				containerCfgApplier,
				iptablesMgr,
				&netns.Execer{},
			),
			propManager,
			iptables.NewPortForwarder(runner),
			portPool,
		)
	case "ducati":
		return &ducati.Ducati{}
	default:
		log.Fatal("failed-to-select-network-module", fmt.Errorf("unknown network module %q", networkModule))
		return nil
	}
}

func wireVolumeCreator(logger lager.Logger, graphRoot string) *rootfs_provider.CakeOrdinator {
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

	graphRetrier := &retrier.Retrier{
		Timeout:         100 * time.Second,
		PollingInterval: 500 * time.Millisecond,
		Clock:           clock.NewClock(),
	}

	quotaedGraphDriver := &quotaed_aufs.QuotaedDriver{
		GraphDriver: dockerGraphDriver,
		Unmount:     quotaed_aufs.Unmount,
		BackingStoreMgr: &quotaed_aufs.BackingStore{
			RootPath: backingStoresPath,
			Logger:   logger.Session("backing-store-mgr"),
		},
		LoopMounter: &quotaed_aufs.Loop{
			Retrier: graphRetrier,
			Logger:  logger.Session("loop-mounter"),
		},
		Retrier:  graphRetrier,
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
			DefaultRootFSPath: *rootFSPath,
			IDProvider:        repository_fetcher.LayerIDProvider{},
		},
	}

	maxId := sysinfo.Min(sysinfo.MustGetMaxValidUID(), sysinfo.MustGetMaxValidGID())
	mappingList := rootfs_provider.MappingList{
		{
			FromID: 0,
			ToID:   maxId,
			Size:   1,
		},
		{
			FromID: 1,
			ToID:   1,
			Size:   maxId - 1,
		},
	}

	rootFSNamespacer := &rootfs_provider.UidNamespacer{
		Logger: logger,
		Translator: rootfs_provider.NewUidTranslator(
			mappingList, // uid
			mappingList, // gid
		),
	}

	layerCreator := rootfs_provider.NewLayerCreator(
		cake, rootfs_provider.SimpleVolumeCreator{}, rootFSNamespacer)

	cakeOrdinator := rootfs_provider.NewCakeOrdinator(
		cake, repoFetcher, layerCreator, nil, logger.Session("cake-ordinator"),
	)
	return cakeOrdinator
}

func wireContainerizer(log lager.Logger, depotPath, iodaemonPath, defaultRootFSPath string) *rundmc.Containerizer {
	depot := depot.New(depotPath)

	startCheck := rundmc.StartChecker{Expect: "Pid 1 Running", Timeout: 3 * time.Second}

	runcrunner := runrunc.New(
		process_tracker.New(path.Join(os.TempDir(), fmt.Sprintf("garden-%s", *tag), "processes"), iodaemonPath, linux_command_runner.New()),
		linux_command_runner.New(),
		wireUidGenerator(),
		goci.RuncBinary("runc"),
	)

	baseBundle := goci.Bundle().
		WithNamespaces(PrivilegedContainerNamespaces...).
		WithResources(&specs.Resources{}).
		WithMounts(goci.Mount{Name: "proc", Type: "proc", Source: "proc", Destination: "/proc"}).
		WithRootFS(defaultRootFSPath).
		WithProcess(goci.Process("/bin/sh", "-c", `echo "Pid 1 Running"; read x`)).
		WithDevices(specs.Device{Path: "/dev/null", Type: 'c', Major: 1, Minor: 3, UID: 0, GID: 0, Permissions: "rwm", FileMode: 0666})

	return rundmc.New(depot, &rundmc.BundleTemplate{Bndl: baseBundle}, runcrunner, startCheck)
}

func missing(flagName string) {
	println("missing " + flagName)
	println()
	flag.Usage()

	os.Exit(1)
}

func parseExternalIP(ip string) (net.IP, error) {
	if *externalIP == "" {
		localIP, err := localip.LocalIP()
		if err != nil {
			return nil, fmt.Errorf("Couldn't determine local IP to use for -externalIP parameter. You can use the -externalIP flag to pass an external IP")
		}
		externalIP = &localIP
	}

	externalIPAddr := net.ParseIP(*externalIP)
	if externalIPAddr == nil {
		return nil, fmt.Errorf("Value of -externalIP %s could not be converted to an IP", *externalIP)
	}
	return externalIPAddr, nil
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

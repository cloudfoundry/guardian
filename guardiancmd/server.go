package guardiancmd

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/guardian/bindata"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/imageplugin"
	"code.cloudfoundry.org/guardian/kawasaki"
	kawasakifactory "code.cloudfoundry.org/guardian/kawasaki/factory"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/kawasaki/mtu"
	"code.cloudfoundry.org/guardian/kawasaki/ports"
	"code.cloudfoundry.org/guardian/kawasaki/subnets"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/guardian/metrics"
	"code.cloudfoundry.org/guardian/netplugin"
	locksmithpkg "code.cloudfoundry.org/guardian/pkg/locksmith"
	"code.cloudfoundry.org/guardian/properties"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/peas/privchecker"
	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	"code.cloudfoundry.org/guardian/rundmc/processes"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/pid"
	"code.cloudfoundry.org/guardian/rundmc/stopper"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/sysinfo"
	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/dropsonde"
	"github.com/docker/docker/pkg/reexec"
	"github.com/eapache/go-resiliency/retrier"
	uuid "github.com/nu7hatch/gouuid"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/sigmon"
)

// These are the maximum caps an unprivileged container process ever gets
// (it may get less if the user is not root, see NonRootMaxCaps)
var unprivilegedMaxCaps = []string{
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
var privilegedMaxCaps = []string{
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
var nonRootMaxCaps = append(unprivilegedMaxCaps, "CAP_SYS_ADMIN")

var PrivilegedContainerNamespaces = []specs.LinuxNamespace{
	goci.NetworkNamespace, goci.PIDNamespace, goci.UTSNamespace, goci.IPCNamespace, goci.MountNamespace,
}

var (
	worldReadWrite = os.FileMode(0666)
	fuseDevice     = specs.LinuxDevice{
		Path:     "/dev/fuse",
		Type:     "c",
		Major:    10,
		Minor:    229,
		FileMode: &worldReadWrite,
	}
	allowedDevices = []specs.LinuxDeviceCgroup{
		// runc allows these
		{Access: "m", Type: "c", Major: deviceWildcard(), Minor: deviceWildcard(), Allow: true},
		{Access: "m", Type: "b", Major: deviceWildcard(), Minor: deviceWildcard(), Allow: true},
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(3), Allow: true},          // /dev/null
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(8), Allow: true},          // /dev/random
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(7), Allow: true},          // /dev/full
		{Access: "rwm", Type: "c", Major: intRef(5), Minor: intRef(0), Allow: true},          // /dev/tty
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(5), Allow: true},          // /dev/zero
		{Access: "rwm", Type: "c", Major: intRef(1), Minor: intRef(9), Allow: true},          // /dev/urandom
		{Access: "rwm", Type: "c", Major: intRef(5), Minor: intRef(1), Allow: true},          // /dev/console
		{Access: "rwm", Type: "c", Major: intRef(136), Minor: deviceWildcard(), Allow: true}, // /dev/pts/*
		{Access: "rwm", Type: "c", Major: intRef(5), Minor: intRef(2), Allow: true},          // /dev/ptmx
		{Access: "rwm", Type: "c", Major: intRef(10), Minor: intRef(200), Allow: true},       // /dev/net/tun

		// We allow these
		{Access: "rwm", Type: fuseDevice.Type, Major: intRef(fuseDevice.Major), Minor: intRef(fuseDevice.Minor), Allow: true},
	}
)

type ServerCommand struct {
	*CommonCommand
}

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

func (cmd *ServerCommand) Execute([]string) error {
	// gdn can be compiled for one of two possible run "modes"
	// 1. all-in-one    - this is meant for standalone deployments
	// 2. bosh-deployed - this is meant for deployment via BOSH
	// when compiling an all-in-one gdn, the bindata package will contain a
	// number of compiled assets (e.g. iptables, runc, etc.), thus we check to
	// see if we have any compiled assets here and perform additional setup
	// (e.g. updating bin paths to point to the compiled assets) if required
	if len(bindata.AssetNames()) > 0 {
		depotDir := cmd.Containers.Dir
		err := os.MkdirAll(depotDir, 0755)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		restoredAssetsDir, err := restoreUnversionedAssets(cmd.Bin.AssetsDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		cmd.Runtime.Plugin = filepath.Join(restoredAssetsDir, "bin", "runc")
		cmd.Bin.Dadoo = FileFlag(filepath.Join(restoredAssetsDir, "bin", "dadoo"))
		cmd.Bin.Init = FileFlag(filepath.Join(restoredAssetsDir, "bin", "init"))
		cmd.Bin.NSTar = FileFlag(filepath.Join(restoredAssetsDir, "bin", "nstar"))
		cmd.Bin.Tar = FileFlag(filepath.Join(restoredAssetsDir, "bin", "tar"))
		cmd.Bin.IPTables = FileFlag(filepath.Join(restoredAssetsDir, "sbin", "iptables"))
		cmd.Bin.IPTablesRestore = FileFlag(filepath.Join(restoredAssetsDir, "sbin", "iptables-restore"))

		if !cmd.Image.NoPlugin {
			if cmd.Image.Plugin == "" {
				cmd.Image.Plugin = FileFlag(filepath.Join(restoredAssetsDir, "bin", "grootfs"))
				cmd.Image.PluginExtraArgs = append([]string{
					"--store", "/var/lib/grootfs/store",
					"--tardis-bin", FileFlag(filepath.Join(restoredAssetsDir, "bin", "tardis")).Path(),
					"--log-level", cmd.Logger.LogLevel,
				}, cmd.Image.PluginExtraArgs...)
			}

			if cmd.Image.PrivilegedPlugin == "" {
				cmd.Image.PrivilegedPlugin = FileFlag(filepath.Join(restoredAssetsDir, "bin", "grootfs"))
				cmd.Image.PrivilegedPluginExtraArgs = append([]string{
					"--store", "/var/lib/grootfs/store-privileged",
					"--tardis-bin", FileFlag(filepath.Join(restoredAssetsDir, "bin", "tardis")).Path(),
					"--log-level", cmd.Logger.LogLevel,
				}, cmd.Image.PrivilegedPluginExtraArgs...)
			}

			maxId := mustGetMaxValidUID()

			initStoreCmd := newInitStoreCommand(cmd.Image.Plugin.Path(), cmd.Image.PluginExtraArgs)
			initStoreCmd.Args = append(initStoreCmd.Args,
				"--uid-mapping", fmt.Sprintf("0:%d:1", maxId),
				"--uid-mapping", fmt.Sprintf("1:1:%d", maxId-1),
				"--gid-mapping", fmt.Sprintf("0:%d:1", maxId),
				"--gid-mapping", fmt.Sprintf("1:1:%d", maxId-1))
			runCommand(initStoreCmd)

			privInitStoreCmd := newInitStoreCommand(cmd.Image.PrivilegedPlugin.Path(), cmd.Image.PrivilegedPluginExtraArgs)
			runCommand(privInitStoreCmd)
		}
	}

	return <-ifrit.Invoke(sigmon.New(cmd)).Wait()
}

func newInitStoreCommand(pluginPath string, pluginGlobalArgs []string) *exec.Cmd {
	return exec.Command(pluginPath, append(pluginGlobalArgs, "init-store", "--store-size-bytes", strconv.FormatInt(10*1024*1024*1024, 10))...)
}

func runCommand(cmd *exec.Cmd) {
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Err: %v Output: %s", err, string(output))
		os.Exit(1)
	}
}

func runningAsRoot() bool {
	return os.Geteuid() == 0
}

func restoreUnversionedAssets(assetsDir string) (string, error) {
	linuxAssetsDir := filepath.Join(assetsDir, "linux")

	_, err := os.Stat(linuxAssetsDir)
	if err == nil {
		return linuxAssetsDir, nil
	}

	err = bindata.RestoreAssets(assetsDir, "linux")
	if err != nil {
		return "", err
	}

	return linuxAssetsDir, nil
}

func (cmd *ServerCommand) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	logger, reconfigurableSink := cmd.Logger.Logger("guardian")
	wiring, err := cmd.createWiring(logger)
	if err != nil {
		return err
	}

	backend := &gardener.Gardener{
		UidGenerator:    wireUIDGenerator(),
		BulkStarter:     wiring.Starter,
		SysInfoProvider: sysinfo.NewResourcesProvider(cmd.Containers.Dir),
		Networker:       wiring.Networker,
		Volumizer:       wiring.Volumizer,
		Containerizer:   wiring.Containerizer,
		PropertyManager: wiring.PropertiesManager,
		MaxContainers:   cmd.Limits.MaxContainers,
		Restorer:        wiring.Restorer,
		PeaCleaner:      wiring.PeaCleaner,

		// We want to be able to disable privileged containers independently of
		// whether or not gdn is running as root.
		AllowPrivilgedContainers: !cmd.Containers.DisablePrivilgedContainers,

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

	metricsProvider := cmd.wireMetricsProvider(logger)

	debugServerMetrics := map[string]func() int{
		"numCPUS":       metricsProvider.NumCPU,
		"numGoRoutines": metricsProvider.NumGoroutine,
		"loopDevices":   metricsProvider.LoopDevices,
		"backingStores": metricsProvider.BackingStores,
		"depotDirs":     metricsProvider.DepotDirs,
	}

	periodicMetronMetrics := map[string]func() int{
		"DepotDirs": metricsProvider.DepotDirs,
	}

	metronNotifier := cmd.wireMetronNotifier(logger, periodicMetronMetrics)
	metronNotifier.Start()

	if cmd.Server.DebugBindIP != nil {
		addr := fmt.Sprintf("%s:%d", cmd.Server.DebugBindIP.IP(), cmd.Server.DebugBindPort)
		metrics.StartDebugServer(addr, reconfigurableSink, debugServerMetrics)
	}

	if err := backend.Start(); err != nil {
		logger.Error("starting-guardian-backend", err)
		return err
	}
	if err := gardenServer.SetupBomberman(); err != nil {
		logger.Error("setting-up-bomberman", err)
		return err
	}
	if err := startServer(gardenServer, logger); err != nil {
		return err
	}

	close(ready)

	logger.Info("started", lager.Data{
		"network": listenNetwork,
		"addr":    listenAddr,
	})

	<-signals

	gardenServer.Stop()

	cmd.saveProperties(logger, cmd.Containers.PropertiesPath, wiring.PropertiesManager)

	portPoolState := wiring.PortPool.RefreshState()
	ports.SaveState(cmd.Network.PortPoolPropertiesPath, portPoolState)

	return nil
}

func (cmd *ServerCommand) wirePeaCleaner(factory GardenFactory, volumizer gardener.Volumizer) gardener.PeaCleaner {
	cmdRunner := factory.CommandRunner()
	runcLogRunner := runrunc.NewLogRunner(cmdRunner, runrunc.LogDir(os.TempDir()).GenerateLogFile)
	runcBinary := goci.RuncBinary{Path: cmd.Runtime.Plugin, Root: cmd.computeRuncRoot()}

	runcStater := runrunc.NewStater(runcLogRunner, runcBinary)
	runcDeleter := runrunc.NewDeleter(runcLogRunner, runcBinary, runcStater)
	return peas.NewPeaCleaner(runcDeleter, volumizer, cmd.Containers.Dir)
}

func (cmd *ServerCommand) calculateDefaultMappingLengths(containerRootUID, containerRootGID int) {
	if cmd.Containers.UIDMapLength == 0 {
		cmd.Containers.UIDMapLength = uint32(containerRootUID) - cmd.Containers.UIDMapStart
	}

	if cmd.Containers.GIDMapLength == 0 {
		cmd.Containers.GIDMapLength = uint32(containerRootGID) - cmd.Containers.GIDMapStart
	}
}

func wireUIDGenerator() gardener.UidGeneratorFunc {
	return gardener.UidGeneratorFunc(func() string { return mustStringify(uuid.NewV4()) })
}

func startServer(gardenServer *server.GardenServer, logger lager.Logger) error {
	socketFDStr := os.Getenv("SOCKET2ME_FD")
	if socketFDStr == "" {
		go func() {
			if err := gardenServer.ListenAndServe(); err != nil {
				logger.Fatal("failed-to-start-server", err)
			}
		}()
		return nil
	}

	socketFD, err := strconv.Atoi(socketFDStr)
	if err != nil {
		return err
	}

	if err = ensureServerSocketDoesNotLeak(uintptr(socketFD)); err != nil {
		logger.Error("failed-to-set-cloexec-on-server-socket", err)
		return err
	}

	listener, err := net.FileListener(os.NewFile(uintptr(socketFD), fmt.Sprintf("/proc/self/fd/%d", socketFD)))
	if err != nil {
		logger.Error("failed-to-listen-on-socket-fd", err)
		return err
	}

	go func() {
		if err := gardenServer.Serve(listener); err != nil {
			logger.Fatal("failed-to-start-server", err)
		}
	}()

	return nil
}

func (cmd *ServerCommand) loadProperties(logger lager.Logger, propertiesPath string) (*properties.Manager, error) {
	propManager, err := properties.Load(propertiesPath)
	if err != nil {
		logger.Error("failed-to-load-properties", err, lager.Data{"propertiesPath": propertiesPath})
		return &properties.Manager{}, err
	}

	return propManager, nil
}

func (cmd *ServerCommand) saveProperties(logger lager.Logger, propertiesPath string, propManager *properties.Manager) {
	if propertiesPath != "" {
		err := properties.Save(propertiesPath, propManager)
		if err != nil {
			logger.Error("failed-to-save-properties", err, lager.Data{"propertiesPath": propertiesPath})
		}
	}
}

func (cmd *ServerCommand) wirePortPool(logger lager.Logger) (*ports.PortPool, error) {
	portPoolState, err := ports.LoadState(cmd.Network.PortPoolPropertiesPath)
	if err != nil {
		if _, ok := err.(ports.StateFileNotFoundError); ok {
			logger.Info("no-port-pool-state-to-recover-starting-clean")
		} else {
			logger.Error("failed-to-parse-port-pool-properties", err)
		}
	}

	portPool, err := ports.NewPool(
		cmd.Network.PortPoolStart,
		cmd.Network.PortPoolSize,
		portPoolState,
	)
	if err != nil {
		return nil, fmt.Errorf("invalid pool range: %s", err)
	}
	return portPool, nil
}

func (cmd *ServerCommand) wireDepot(bundleGenerator depot.BundleGenerator, bundleSaver depot.BundleSaver, bindMountSourceCreator depot.BindMountSourceCreator) *depot.DirectoryDepot {
	return depot.New(cmd.Containers.Dir, bundleGenerator, bundleSaver, bindMountSourceCreator)
}

func (cmd *ServerCommand) wireNetworker(log lager.Logger, factory GardenFactory, propManager kawasaki.ConfigStore, portPool *ports.PortPool) (gardener.Networker, gardener.Starter, error) {
	externalIP, err := defaultExternalIP(cmd.Network.ExternalIP)
	if err != nil {
		return nil, nil, err
	}

	dnsServers := extractIPs(cmd.Network.DNSServers)
	additionalDNSServers := extractIPs(cmd.Network.AdditionalDNSServers)

	if cmd.Network.Plugin.Path() != "" {
		resolvConfigurer := factory.WireResolvConfigurer()
		externalNetworker := netplugin.New(
			factory.CommandRunner(),
			propManager,
			externalIP,
			dnsServers,
			additionalDNSServers,
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
	locksmith := &locksmithpkg.FileSystem{}

	iptRunner := &logging.Runner{CommandRunner: factory.CommandRunner(), Logger: log.Session("iptables-runner")}
	ipTables := iptables.New(cmd.Bin.IPTables.Path(), cmd.Bin.IPTablesRestore.Path(), iptRunner, locksmith, chainPrefix)
	nonLoggingIPTables := iptables.New(cmd.Bin.IPTables.Path(), cmd.Bin.IPTablesRestore.Path(), factory.CommandRunner(), locksmith, chainPrefix)
	ipTablesStarter := iptables.NewStarter(nonLoggingIPTables, cmd.Network.AllowHostAccess, interfacePrefix, denyNetworksList, cmd.Containers.DestroyContainersOnStartup, log)
	ruleTranslator := iptables.NewRuleTranslator()

	containerMtu := cmd.Network.Mtu
	if containerMtu == 0 {
		containerMtu, err = mtu.MTU(externalIP.String())
		if err != nil {
			return nil, nil, err
		}
	}

	networker := kawasaki.New(
		kawasaki.SpecParserFunc(kawasaki.ParseSpec),
		subnets.NewPool(cmd.Network.Pool.CIDR()),
		kawasaki.NewConfigCreator(idGenerator, interfacePrefix, chainPrefix, externalIP, dnsServers, additionalDNSServers, cmd.Network.AdditionalHostEntries, containerMtu),
		propManager,
		kawasakifactory.NewDefaultConfigurer(ipTables, cmd.Containers.Dir),
		portPool,
		iptables.NewPortForwarder(ipTables),
		iptables.NewFirewallOpener(ruleTranslator, ipTables),
	)

	return networker, ipTablesStarter, nil
}

func (cmd *ServerCommand) wireImagePlugin(commandRunner commandrunner.CommandRunner, uid, gid int) gardener.Volumizer {
	var unprivilegedCommandCreator imageplugin.CommandCreator = &imageplugin.NotImplementedCommandCreator{
		Err: errors.New("no image_plugin provided"),
	}

	var privilegedCommandCreator imageplugin.CommandCreator = &imageplugin.NotImplementedCommandCreator{
		Err: errors.New("no privileged_image_plugin provided"),
	}

	if cmd.Image.Plugin.Path() != "" {
		unprivilegedCommandCreator = &imageplugin.DefaultCommandCreator{
			BinPath:   cmd.Image.Plugin.Path(),
			ExtraArgs: cmd.Image.PluginExtraArgs,
		}
	}

	if cmd.Image.PrivilegedPlugin.Path() != "" {
		privilegedCommandCreator = &imageplugin.DefaultCommandCreator{
			BinPath:   cmd.Image.PrivilegedPlugin.Path(),
			ExtraArgs: cmd.Image.PrivilegedPluginExtraArgs,
		}
	}

	imagePlugin := &imageplugin.ImagePlugin{
		UnprivilegedCommandCreator: unprivilegedCommandCreator,
		PrivilegedCommandCreator:   privilegedCommandCreator,
		ImageSpecCreator:           imageplugin.NewOCIImageSpecCreator(cmd.Containers.Dir),
		CommandRunner:              commandRunner,
		DefaultRootfs:              cmd.Containers.DefaultRootFS,
	}

	return gardener.NewVolumeProvider(imagePlugin, imagePlugin, gardener.CommandFactory(preparerootfs.Command), commandRunner, uid, gid)
}

func (cmd *ServerCommand) wireContainerizer(log lager.Logger, factory GardenFactory,
	properties gardener.PropertyManager, volumizer peas.Volumizer, peaCleaner gardener.PeaCleaner) (*rundmc.Containerizer, error) {

	initMount, initPath := initBindMountAndPath(cmd.Bin.Init.Path())

	defaultMounts := append(defaultBindMounts(), initMount)
	privilegedMounts := append(defaultMounts, privilegedMounts()...)
	unprivilegedMounts := append(defaultMounts, unprivilegedMounts()...)

	// TODO centralize knowledge of garden -> runc capability schema translation
	baseProcess := specs.Process{
		Capabilities: &specs.LinuxCapabilities{
			Effective:   unprivilegedMaxCaps,
			Bounding:    unprivilegedMaxCaps,
			Inheritable: unprivilegedMaxCaps,
			Permitted:   unprivilegedMaxCaps,
		},
		Args:        []string{initPath},
		Cwd:         "/",
		ConsoleSize: &specs.Box{},
	}

	baseBundle := goci.Bundle().
		WithNamespaces(PrivilegedContainerNamespaces...).
		WithRootFS(cmd.Containers.DefaultRootFS).
		WithProcess(baseProcess).
		WithRootFSPropagation("private")

	uidMappings, gidMappings := cmd.idMappings()
	unprivilegedBundle := baseBundle.
		WithNamespace(goci.UserNamespace).
		WithUIDMappings(uidMappings...).
		WithGIDMappings(gidMappings...).
		WithMounts(unprivilegedMounts...).
		WithMaskedPaths(defaultMaskedPaths())

	unprivilegedBundle.Spec.Linux.Seccomp = seccomp
	if cmd.Containers.ApparmorProfile != "" {
		unprivilegedBundle = unprivilegedBundle.WithApparmorProfile(cmd.Containers.ApparmorProfile)
	}
	privilegedBundle := baseBundle.
		WithMounts(privilegedMounts...).
		WithDevices(getPrivilegedDevices()...).
		WithCapabilities(privilegedMaxCaps...).
		WithDeviceRestrictions(append(
			[]specs.LinuxDeviceCgroup{{Allow: false, Access: "rwm"}},
			allowedDevices...,
		))

	log.Debug("base-bundles", lager.Data{
		"privileged":   privilegedBundle,
		"unprivileged": unprivilegedBundle,
	})

	cgroupRootPath := "garden"
	if cmd.Server.Tag != "" {
		cgroupRootPath = fmt.Sprintf("%s-%s", cgroupRootPath, cmd.Server.Tag)
	}

	bundleRules := []rundmc.BundlerRule{
		bundlerules.Base{
			PrivilegedBase:   privilegedBundle,
			UnprivilegedBase: unprivilegedBundle,
		},
		bundlerules.Namespaces{},
		bundlerules.CGroupPath{
			Path: cgroupRootPath,
		},
		wireMounts(),
		bundlerules.Env{},
		bundlerules.Hostname{},
		bundlerules.Windows{},
		bundlerules.RootFS{},
		bundlerules.Limits{
			CpuQuotaPerShare: cmd.Limits.CPUQuotaPerShare,
			TCPMemoryLimit:   int64(cmd.Limits.TCPMemoryLimit),
			BlockIOWeight:    cmd.Limits.DefaultBlockIOWeight,
			DisableSwapLimit: cmd.Limits.DisableSwapLimit,
		},
	}
	template := &rundmc.BundleTemplate{Rules: bundleRules}

	bundleSaver := &goci.BundleSaver{}
	bindMountSourceCreator := wireBindMountSourceCreator(uidMappings, gidMappings)
	depot := cmd.wireDepot(template, bundleSaver, bindMountSourceCreator)

	bndlLoader := &goci.BndlLoader{}
	processBuilder := processes.NewBuilder(wireEnvFunc(), nonRootMaxCaps)

	cmdRunner := factory.CommandRunner()
	runcLogRunner := runrunc.NewLogRunner(cmdRunner, runrunc.LogDir(os.TempDir()).GenerateLogFile)
	runcRoot := cmd.computeRuncRoot()
	runcBinary := goci.RuncBinary{Path: cmd.Runtime.Plugin, Root: runcRoot}

	pidFileReader := wirePidfileReader()
	privilegeChecker := &privchecker.PrivilegeChecker{BundleLoader: bndlLoader}
	runcStater := runrunc.NewStater(runcLogRunner, runcBinary)
	runcDeleter := runrunc.NewDeleter(runcLogRunner, runcBinary, runcStater)

	var runner rundmc.OCIRuntime
	var pidGetter peas.ProcessPidGetter
	var peaPidGetter peas.ProcessPidGetter = &pid.ContainerPidGetter{Depot: depot, PidFileReader: pidFileReader}
	var peaCreator *peas.PeaCreator

	userLookupper := users.LookupFunc(users.LookupUser)

	wireExecerFunc := func(pidGetter runrunc.PidGetter) *runrunc.Execer {
		return runrunc.NewExecer(bndlLoader, processBuilder, factory.WireMkdirer(),
			userLookupper, factory.WireExecRunner("exec", runcRoot, uint32(uidMappings.Map(0)), uint32(gidMappings.Map(0))), pidGetter)
	}

	statser := runrunc.NewStatser(runcLogRunner, runcBinary)

	var useNestedCgroups bool
	var execRunner runrunc.ExecRunner = factory.WireExecRunner("run", runcRoot, uint32(uidMappings.Map(0)), uint32(gidMappings.Map(0)))
	if cmd.useContainerd() {
		var err error
		var peaRunner *runcontainerd.RunContainerPea
		runner, peaRunner, pidGetter, err = factory.WireContainerd(bndlLoader, processBuilder, userLookupper, wireExecerFunc, statser)
		if err != nil {
			return nil, err
		}

		if cmd.Containerd.UseContainerdForProcesses {
			peaPidGetter = pidGetter
			execRunner = peaRunner
			useNestedCgroups = true
		}
	} else {
		pidGetter = &pid.ContainerPidGetter{Depot: depot, PidFileReader: pidFileReader}
		runner = runrunc.New(
			cmdRunner,
			runcLogRunner,
			runcBinary,
			cmd.Runtime.PluginExtraArgs,
			wireExecerFunc(pidGetter),
			statser,
		)
	}

	eventStore := rundmc.NewEventStore(properties)
	stateStore := rundmc.NewStateStore(properties)

	peaCreator = &peas.PeaCreator{
		Volumizer:              volumizer,
		PidGetter:              pidGetter,
		PrivilegedGetter:       privilegeChecker,
		BindMountSourceCreator: bindMountSourceCreator,
		BundleGenerator:        template,
		ProcessBuilder:         processBuilder,
		BundleSaver:            bundleSaver,
		ExecRunner:             execRunner,
		RuncDeleter:            runcDeleter,
		PeaCleaner:             peaCleaner,
		NestedCgroups:          useNestedCgroups,
	}

	peaUsernameResolver := &peas.PeaUsernameResolver{
		PidGetter:     peaPidGetter,
		PeaCreator:    peaCreator,
		Loader:        bndlLoader,
		UserLookupper: users.LookupFunc(users.LookupUser),
	}

	nstar := rundmc.NewNstarRunner(cmd.Bin.NSTar.Path(), cmd.Bin.Tar.Path(), cmdRunner)
	stopper := stopper.New(stopper.NewRuncStateCgroupPathResolver(runcRoot), nil, retrier.New(retrier.ConstantBackoff(10, 1*time.Second), nil))
	return rundmc.New(depot, runner, bndlLoader, nstar, stopper, eventStore, stateStore, factory.WireRootfsFileCreator(), peaCreator, peaUsernameResolver, cmd.Metrics.CPUEntitlementPerShare), nil
}

func (cmd *ServerCommand) wireMetricsProvider(log lager.Logger) *metrics.MetricsProvider {
	return metrics.NewMetricsProvider(log, cmd.Containers.Dir)
}

func (cmd *ServerCommand) wireMetronNotifier(log lager.Logger, metricsProvider metrics.Metrics) *metrics.PeriodicMetronNotifier {
	return metrics.NewPeriodicMetronNotifier(
		log, metricsProvider, cmd.Metrics.EmissionInterval, clock.NewClock(),
	)
}

func (cmd *ServerCommand) initializeDropsonde(log lager.Logger) {
	err := dropsonde.Initialize(cmd.Metrics.DropsondeDestination, cmd.Metrics.DropsondeOrigin)
	if err != nil {
		log.Error("failed to initialize dropsonde", err)
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

func deviceWildcard() *int64 {
	return intRef(-1)
}

func intRef(i int64) *int64 {
	return &i
}

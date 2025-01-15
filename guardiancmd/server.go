package guardiancmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/guardian/bindata"
	"code.cloudfoundry.org/guardian/kawasaki/ports"
	"code.cloudfoundry.org/guardian/metrics"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/lager/v3"
	"github.com/cloudfoundry/dropsonde"
	"github.com/moby/sys/reexec"
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

			maxID := mustGetMaxValidUID()

			initStoreCmd := newInitStoreCommand(cmd.Image.Plugin.Path(), cmd.Image.PluginExtraArgs)
			initStoreCmd.Args = append(initStoreCmd.Args,
				"--uid-mapping", fmt.Sprintf("0:%d:1", maxID),
				"--uid-mapping", fmt.Sprintf("1:1:%d", maxID-1),
				"--gid-mapping", fmt.Sprintf("0:%d:1", maxID),
				"--gid-mapping", fmt.Sprintf("1:1:%d", maxID-1))
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
	metricsProvider := cmd.wireMetricsProvider(logger)
	wiring, err := cmd.createWiring(logger, metricsProvider)
	if err != nil {
		return err
	}

	backend := cmd.createGardener(wiring)

	var listenNetwork, listenAddr string
	if cmd.Server.BindIP != nil {
		listenNetwork = "tcp"
		listenAddr = fmt.Sprintf("%s:%d", cmd.Server.BindIP.IP(), cmd.Server.BindPort)
	} else {
		listenNetwork = "unix"
		listenAddr = cmd.Server.BindSocket
	}

	gardenServer := server.New(listenNetwork, listenAddr, cmd.Containers.DefaultGraceTime, cmd.Server.ReadHeaderTimeout, backend, logger.Session("api"))
	// listen on the socket prior to serving, to ensure unix socket files are created and the healthcheck
	// process can launch while the backend runs its cleanup. However, don't serve requests in gardenServer
	// until the cleanup is complete
	gardenListener, err := gardenServer.Listen()
	if err != nil {
		logger.Error("listening-on-socket", err)
		return err
	}

	cmd.initializeDropsonde(logger)

	debugServerMetrics := map[string]func() int{
		"numCPUS":       metricsProvider.NumCPU,
		"numGoRoutines": metricsProvider.NumGoroutine,
		"loopDevices":   metricsProvider.LoopDevices,
		"backingStores": metricsProvider.BackingStores,
		"depotDirs":     metricsProvider.DepotDirs,
	}

	periodicMetronMetrics := map[string]func() int{
		"DepotDirs":            metricsProvider.DepotDirs,
		"UnkillableContainers": metricsProvider.UnkillableContainers,
	}

	metronNotifier := cmd.wireMetronNotifier(logger, periodicMetronMetrics)
	metronNotifier.Start()

	if cmd.Server.DebugBindIP != nil {
		addr := fmt.Sprintf("%s:%d", cmd.Server.DebugBindIP.IP(), cmd.Server.DebugBindPort)
		_, err := metrics.StartDebugServer(addr, reconfigurableSink, debugServerMetrics)
		if err != nil {
			logger.Debug("failed-to-start-debug-server", lager.Data{"error": err})
		}
	}

	if err := backend.Start(); err != nil {
		logger.Error("starting-guardian-backend", err)
		return err
	}

	if err := gardenServer.SetupBomberman(); err != nil {
		logger.Error("setting-up-bomberman", err)
		return err
	}

	services, err := cmd.wireServices(logger, wiring.Containerizer, wiring.SysInfoProvider, wiring.CpuEntitlementPerShare)
	if err != nil {
		return err
	}

	startServices(services)
	if err := startServer(gardenServer, gardenListener, logger); err != nil {
		return err
	}

	close(ready)

	logger.Info("started", lager.Data{
		"network": listenNetwork,
		"addr":    listenAddr,
	})

	<-signals

	if err := gardenServer.Stop(); err != nil {
		logger.Error("stopping-garden-server", err)
	}
	stopServices(services)

	cmd.saveProperties(logger, cmd.Containers.PropertiesPath, wiring.PropertiesManager)

	portPoolState := wiring.PortPool.RefreshState()
	err = ports.SaveState(cmd.Network.PortPoolPropertiesPath, portPoolState)
	if err != nil {
		logger.Debug("failed-saving-state", lager.Data{"error": err})
	}

	return nil
}

func startServer(gardenServer *server.GardenServer, gdnListener net.Listener, logger lager.Logger) error {
	socketFDStr := os.Getenv("SOCKET2ME_FD")
	if socketFDStr == "" {
		go func() {
			if err := gardenServer.Serve(gdnListener); err != nil {
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

func (cmd *ServerCommand) initializeDropsonde(log lager.Logger) {
	err := dropsonde.Initialize(cmd.Metrics.DropsondeDestination, cmd.Metrics.DropsondeOrigin)
	if err != nil {
		log.Error("failed to initialize dropsonde", err)
	}
}

func (cmd *ServerCommand) wireServices(log lager.Logger, containerizer *rundmc.Containerizer, memoryProvider throttle.MemoryProvider, cpuEntitlementPerShare float64) ([]Service, error) {
	services := []Service{}

	if cmd.CPUThrottling.Enabled {
		cpuThrottling, err := cmd.wireCpuThrottlingService(log, containerizer, memoryProvider, cpuEntitlementPerShare)
		if err != nil {
			return nil, err
		}

		services = append(services, cpuThrottling)
	}

	return services, nil
}

func startServices(services []Service) {
	for _, s := range services {
		s.Start()
	}
}

func stopServices(services []Service) {
	for _, s := range services {
		s.Stop()
	}
}

func mustStringify(s interface{}, e error) string {
	if e != nil {
		panic(e)
	}

	return fmt.Sprintf("%s", s)
}

func deviceWildcard() *int64 {
	return intRef(-1)
}

func intRef(i int64) *int64 {
	return &i
}

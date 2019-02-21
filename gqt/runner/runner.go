package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/guardian/gqt/cgrouper"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	multierror "github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type GdnRunnerConfig struct {
	TmpDir         string
	User           UserCredential
	ConfigFilePath string

	Socket2meBin        string
	Socket2meSocketPath string
	RuncRoot            string

	// Garden config
	GdnBin                   string
	GrootBin                 string
	TarBin                   string `flag:"tar-bin"`
	InitBin                  string `flag:"init-bin"`
	RuntimePluginBin         string `flag:"runtime-plugin"`
	ImagePluginBin           string `flag:"image-plugin"`
	PrivilegedImagePluginBin string `flag:"privileged-image-plugin"`
	NetworkPluginBin         string `flag:"network-plugin"`
	ExecRunnerBin            string `flag:"dadoo-bin"`
	NSTarBin                 string `flag:"nstar-bin"`

	DefaultRootFS                  string   `flag:"default-rootfs"`
	DepotDir                       string   `flag:"depot"`
	ConsoleSocketsPath             string   `flag:"console-sockets-path"`
	BindIP                         string   `flag:"bind-ip"`
	BindPort                       *int     `flag:"bind-port"`
	BindSocket                     string   `flag:"bind-socket"`
	DenyNetworks                   []string `flag:"deny-network"`
	DefaultBlkioWeight             *uint64  `flag:"default-container-blockio-weight"`
	NetworkPluginExtraArgs         []string `flag:"network-plugin-extra-arg"`
	ImagePluginExtraArgs           []string `flag:"image-plugin-extra-arg"`
	RuntimePluginExtraArgs         []string `flag:"runtime-plugin-extra-arg"`
	PrivilegedImagePluginExtraArgs []string `flag:"privileged-image-plugin-extra-arg"`
	MaxContainers                  *uint64  `flag:"max-containers"`
	DebugIP                        string   `flag:"debug-bind-ip"`
	DebugPort                      *int     `flag:"debug-bind-port"`
	PropertiesPath                 string   `flag:"properties-path"`
	LogLevel                       string   `flag:"log-level"`
	TCPMemoryLimit                 *uint64  `flag:"tcp-memory-limit"`
	CPUQuotaPerShare               *uint64  `flag:"cpu-quota-per-share"`
	IPTablesBin                    string   `flag:"iptables-bin"`
	IPTablesRestoreBin             string   `flag:"iptables-restore-bin"`
	DNSServers                     []string `flag:"dns-server"`
	AdditionalDNSServers           []string `flag:"additional-dns-server"`
	AdditionalHostEntries          []string `flag:"additional-host-entry"`
	MTU                            *int     `flag:"mtu"`
	PortPoolSize                   *int     `flag:"port-pool-size"`
	PortPoolStart                  *int     `flag:"port-pool-start"`
	PortPoolPropertiesPath         string   `flag:"port-pool-properties-path"`
	DestroyContainersOnStartup     *bool    `flag:"destroy-containers-on-startup"`
	DockerRegistry                 string   `flag:"docker-registry"`
	InsecureDockerRegistry         string   `flag:"insecure-docker-registry"`
	AllowHostAccess                *bool    `flag:"allow-host-access"`
	SkipSetup                      *bool    `flag:"skip-setup"`
	UIDMapStart                    *uint32  `flag:"uid-map-start"`
	UIDMapLength                   *uint32  `flag:"uid-map-length"`
	GIDMapStart                    *uint32  `flag:"gid-map-start"`
	GIDMapLength                   *uint32  `flag:"gid-map-length"`
	CleanupProcessDirsOnWait       *bool    `flag:"cleanup-process-dirs-on-wait"`
	DisablePrivilegedContainers    *bool    `flag:"disable-privileged-containers"`
	AppArmor                       string   `flag:"apparmor"`
	Tag                            string   `flag:"tag"`
	NetworkPool                    string   `flag:"network-pool"`
	ContainerdSocket               string   `flag:"containerd-socket"`
	UseContainerdForProcesses      *bool    `flag:"use-containerd-for-processes"`

	StartupExpectedToFail bool
	StorePath             string
	PrivilegedStorePath   string
}

func (c GdnRunnerConfig) connectionInfo() (string, string) {
	if c.Socket2meSocketPath != "" {
		return "unix", c.Socket2meSocketPath
	}
	if c.BindSocket != "" {
		return "unix", c.BindSocket
	}
	return "tcp", fmt.Sprintf("%s:%d", c.BindIP, *c.BindPort)
}

func (c GdnRunnerConfig) toServerFlags() []string {
	gardenArgs := []string{}
	if c.ConfigFilePath != "" {
		gardenArgs = append(gardenArgs, "--config", c.ConfigFilePath)
	}
	gardenArgs = append(gardenArgs, "containerized-server")

	vConf := reflect.ValueOf(c)
	tConf := vConf.Type()
	for i := 0; i < tConf.NumField(); i++ {
		tField := tConf.Field(i)
		flagName, ok := tField.Tag.Lookup("flag")
		if !ok {
			continue
		}

		vField := vConf.Field(i)
		if vField.Kind() != reflect.String && vField.IsNil() {
			continue
		}

		fieldVal := reflect.Indirect(vField).Interface()
		switch v := fieldVal.(type) {
		case string:
			if v != "" {
				gardenArgs = append(gardenArgs, "--"+flagName, v)
			}
			if v == "" && vField.Kind() != reflect.String && !vField.IsNil() {
				gardenArgs = append(gardenArgs, "--"+flagName, "")
			}
		case int, uint64, uint32:
			gardenArgs = append(gardenArgs, "--"+flagName, fmt.Sprintf("%d", v))
		case bool:
			if v {
				gardenArgs = append(gardenArgs, "--"+flagName)
			}
		case []string:
			for _, val := range v {
				gardenArgs = append(gardenArgs, "--"+flagName, val)
			}
		default:
			Fail(fmt.Sprintf("unrecognised field type for field %s", flagName))
		}
	}

	return gardenArgs
}

type Binaries struct {
	Tar                   string `json:"tar,omitempty"`
	Gdn                   string `json:"gdn,omitempty"`
	Groot                 string `json:"groot,omitempty"`
	Tardis                string `json:"tardis,omitempty"`
	Init                  string `json:"init,omitempty"`
	RuntimePlugin         string `json:"runtime_plugin,omitempty"`
	ImagePlugin           string `json:"image_plugin,omitempty"`
	PrivilegedImagePlugin string `json:"privileged_image_plugin,omitempty"`
	NetworkPlugin         string `json:"network_plugin,omitempty"`
	NoopPlugin            string `json:"noop_plugin,omitempty"`
	ExecRunner            string `json:"execrunner,omitempty"`
	NSTar                 string `json:"nstar,omitempty"`
	Socket2me             string `json:"socket2me,omitempty"`
}

type GardenRunner struct {
	*GdnRunnerConfig
	*ginkgomon.Runner
}

func (r *GardenRunner) Setup() {
	r.setupDirsForUser()
}

type RunningGarden struct {
	*GardenRunner
	client.Client

	process ifrit.Process
	Pid     int
	logger  lager.Logger
}

func init() {
}

func DefaultGdnRunnerConfig(binaries Binaries) GdnRunnerConfig {
	var config GdnRunnerConfig
	config.Tag = fmt.Sprintf("%d", GinkgoParallelNode())

	var err error
	config.TmpDir, err = ioutil.TempDir("", fmt.Sprintf("test-garden-%s-", config.Tag))
	Expect(err).NotTo(HaveOccurred())
	Expect(os.Chmod(config.TmpDir, 0777)).To(Succeed())

	config.ConsoleSocketsPath = filepath.Join(config.TmpDir, "console-sockets")
	config.DepotDir = filepath.Join(config.TmpDir, "containers")
	Expect(os.MkdirAll(config.DepotDir, 0755)).To(Succeed())

	if runtime.GOOS == "windows" {
		config.BindIP = "127.0.0.1"
		config.BindPort = intptr(10000 + os.Getpid())
	} else {
		config.BindSocket = fmt.Sprintf("/tmp/garden_%s.sock", config.Tag)
	}

	config.NetworkPool = fmt.Sprintf("10.254.%d.0/22", 4*GinkgoParallelNode())
	config.PortPoolStart = intptr(GinkgoParallelNode() * 7000)

	config.UIDMapStart = uint32ptr(1)
	config.UIDMapLength = uint32ptr(100000)
	config.GIDMapStart = uint32ptr(1)
	config.GIDMapLength = uint32ptr(100000)

	config.StorePath = filepath.Join(config.TmpDir, "groot_store")
	config.PrivilegedStorePath = filepath.Join(config.TmpDir, "groot_privileged_store")

	config.ImagePluginExtraArgs = []string{`"--store"`, config.StorePath, `"--tardis-bin"`, binaries.Tardis, `"--log-level"`, "debug"}
	config.PrivilegedImagePluginExtraArgs = []string{`"--store"`, config.PrivilegedStorePath, `"--tardis-bin"`, binaries.Tardis, `"--log-level"`, "debug"}
	config.LogLevel = "debug"

	return config
}

func NewGardenRunner(config GdnRunnerConfig) *GardenRunner {
	runner := &GardenRunner{
		GdnRunnerConfig: &config,
		Runner: ginkgomon.New(ginkgomon.Config{
			Name:              "guardian",
			AnsiColorCode:     "31m",
			StartCheck:        "",
			StartCheckTimeout: 30 * time.Second,
		}),
	}

	if config.Socket2meSocketPath == "" {
		runner.Command = exec.Command(config.GdnBin, config.toServerFlags()...)
	} else {
		runner.Command = socket2meCommand(config)
	}

	runner.Command.Env = append(
		os.Environ(),
		[]string{
			fmt.Sprintf("TMPDIR=%s", runner.TmpDir),
			fmt.Sprintf("TEMP=%s", runner.TmpDir),
			fmt.Sprintf("TMP=%s", runner.TmpDir),
		}...,
	)
	if config.RuncRoot != "" {
		runner.Command.Env = append(
			os.Environ(),
			[]string{
				fmt.Sprintf("XDG_RUNTIME_DIR=%s", config.RuncRoot),
			}...,
		)
	}
	setUserCredential(runner)

	runner.Setup()

	return runner
}

func Start(config GdnRunnerConfig) *RunningGarden {
	if runtime.GOOS == "linux" {
		initGrootStore(config.ImagePluginBin, config.StorePath, []string{"0:4294967294:1", "1:65536:4294901758"})
		initGrootStore(config.PrivilegedImagePluginBin, config.PrivilegedStorePath, nil)
	}

	runner := NewGardenRunner(config)

	gdn := &RunningGarden{
		GardenRunner: runner,
		logger:       lagertest.NewTestLogger("garden-runner"),
	}

	gdn.process = ifrit.Invoke(runner)
	gdn.Pid = runner.Command.Process.Pid

	// output, err := exec.Command("pgrep", "-P", strconv.Itoa(runner.Command.Process.Pid)).CombinedOutput()
	// Expect(err).NotTo(HaveOccurred(), string(output))
	// gdn.Pid, err = strconv.Atoi(strings.TrimSpace(string(output)))
	// Expect(err).NotTo(HaveOccurred())

	gdn.Client = client.New(connection.New(runner.connectionInfo()))

	if !config.StartupExpectedToFail {
		Eventually(gdn.Ping, time.Second*10).Should(Succeed())

		pgrep := func() error {
			output, err := exec.Command("pgrep", "-P", strconv.Itoa(runner.Command.Process.Pid)).CombinedOutput()
			if err != nil {
				return err
			}
			if len(output) > 0 {
				gdn.Pid, err = strconv.Atoi(strings.TrimSpace(string(output)))
				if err != nil {
					return err
				}
			}
			return nil
		}
		Eventually(pgrep).Should(Succeed())
	}

	return gdn
}

func (r *RunningGarden) Kill() error {
	r.process.Signal(syscall.SIGKILL)
	select {
	case err := <-r.process.Wait():
		return err
	case <-time.After(time.Second * 10):
		r.process.Signal(syscall.SIGKILL)
		return errors.New("timed out waiting for garden to shutdown after 10 seconds")
	}
}

type ErrGardenStop struct {
	error
}

func (r *RunningGarden) DestroyAndStop() error {
	return multierror.Append(
		r.DestroyContainers(),
		r.forceStop(),
		r.Cleanup(),
	).ErrorOrNil()
}

func (r *RunningGarden) forceStop() error {
	if runtime.GOOS == "windows" {
		// Windows doesn't support SIGTERM
		r.Kill()
	} else {
		if err := r.Stop(); err != nil {
			fmt.Printf("error on r.Stop() during forceStop: %s\n", err.Error())
			return ErrGardenStop{error: err}
		}
	}

	if err := r.removeTempDirContentsPreservingGrootFSStores(); err != nil {
		fmt.Printf("error on r.removeTempDirContentsPreservingGrootFSStore() during forceStop: %s\n", err.Error())
		return err
	}

	return nil
}

func (r *RunningGarden) CgroupsRootPath() string {
	return CgroupsRootPath(r.Tag)
}

func CgroupsRootPath(tag string) string {
	return filepath.Join("/tmp", fmt.Sprintf("cgroups-%s", tag))
}

func (r *RunningGarden) CgroupSubsystemPath(subsystem, handle string) string {
	gardenCgroupRelativePath, err := cgrouper.GetCGroup(subsystem)
	Expect(err).NotTo(HaveOccurred())

	return filepath.Join(CgroupsRootPath(r.Tag), subsystem, gardenCgroupRelativePath, "garden-"+r.Tag, handle)
}

func (r *RunningGarden) removeTempDirContentsPreservingGrootFSStores() error {
	tmpDir, err := os.Open(r.TmpDir)
	if err != nil {
		return err
	}
	defer tmpDir.Close()
	tmpDirContents, err := tmpDir.Readdir(0)
	if err != nil {
		return err
	}

	for _, tmpDirChild := range tmpDirContents {
		if !strings.Contains(tmpDirChild.Name(), "store") {
			if err := os.RemoveAll(filepath.Join(r.TmpDir, tmpDirChild.Name())); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *RunningGarden) Stop() error {
	r.process.Signal(syscall.SIGTERM)

	var err error
	for i := 0; i < 5; i++ {
		select {
		case err := <-r.process.Wait():
			return err
		case <-time.After(time.Second * 5):
			r.process.Signal(syscall.SIGTERM)
			err = errors.New("timed out waiting for garden to shutdown after 5 seconds")
		}
	}

	r.process.Signal(syscall.SIGKILL)
	return err
}

func (r *RunningGarden) DestroyContainers() error {
	containers, err := r.Containers(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		if destroyErr := r.Destroy(container.Handle()); destroyErr != nil {
			err = multierror.Append(destroyErr)
		}
	}

	return err
}

type debugVars struct {
	NumGoRoutines int `json:"numGoRoutines"`
}

func (r *RunningGarden) NumGoroutines() (int, error) {
	debugURL := fmt.Sprintf("http://%s:%d/debug/vars", r.DebugIP, *r.DebugPort)
	res, err := http.Get(debugURL)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	var debugVarsData debugVars
	err = decoder.Decode(&debugVarsData)
	if err != nil {
		return 0, err
	}

	return debugVarsData.NumGoRoutines, nil
}

func (r *RunningGarden) StackDump() (string, error) {
	debugURL := fmt.Sprintf("http://%s:%d/debug/pprof/goroutine?debug=2", r.DebugIP, *r.DebugPort)
	res, err := http.Get(debugURL)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	stack, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(stack), nil
}

func intptr(i int) *int {
	return &i
}

func uint32ptr(i uint32) *uint32 {
	return &i
}

func initGrootStore(grootBin, storePath string, idMappings []string) {
	if filepath.Base(grootBin) != "grootfs" {
		// Don't initialise the grootfs store for fake image plugins
		// This is important to prevent loop device leakige!
		return
	}

	initStoreArgs := []string{"--store", storePath, "init-store", "--store-size-bytes", fmt.Sprintf("%d", 2*1024*1024*1024)}
	for _, idMapping := range idMappings {
		initStoreArgs = append(initStoreArgs, "--uid-mapping", idMapping, "--gid-mapping", idMapping)
	}

	initStore := exec.Command(grootBin, initStoreArgs...)
	initStore.Stdout = GinkgoWriter
	initStore.Stderr = GinkgoWriter
	Expect(initStore.Run()).To(Succeed())
}

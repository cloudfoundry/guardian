package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/eapache/go-resiliency/retrier"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

const MNT_DETACH = 0x2

var DataDir string
var TarBin = os.Getenv("GARDEN_TAR_PATH")

type RunningGarden struct {
	client.Client

	runner  GardenRunner
	process ifrit.Process

	debugIP   string
	debugPort int

	Pid int

	Tmpdir string

	DepotDir  string
	DataDir   string
	GraphPath string

	logger lager.Logger
}

type GardenRunner struct {
	*ginkgomon.Runner
	Cmd           *exec.Cmd
	TmpDir        string
	GraphPath     string
	DepotDir      string
	DebugIp       string
	DebugPort     int
	Network, Addr string
}

func init() {
	DataDir = os.Getenv("GARDEN_TEST_GRAPHPATH")
	if DataDir == "" {
		// This must be set outside of the Ginkgo node directory (tmpDir) because
		// otherwise the Concourse worker may run into one of the AUFS kernel
		// module bugs that cause the VM to become unresponsive.
		DataDir = "/tmp/aufs_mount"
	}
}

func NewGardenRunner(bin, initBin, nstarBin, dadooBin, grootfsBin, rootfs, tarBin, network, address string, argv ...string) GardenRunner {
	r := GardenRunner{}

	r.Network = network
	r.Addr = address
	r.TmpDir = filepath.Join(
		os.TempDir(),
		fmt.Sprintf("test-garden-%d", ginkgo.GinkgoParallelNode()),
	)

	r.GraphPath = filepath.Join(DataDir, fmt.Sprintf("node-%d", ginkgo.GinkgoParallelNode()))
	r.DepotDir = filepath.Join(r.TmpDir, "containers")

	MustMountTmpfs(r.GraphPath)

	r.Cmd = cmd(r.TmpDir, r.DepotDir, r.GraphPath, r.Network, r.Addr, bin, initBin, nstarBin, dadooBin, grootfsBin, tarBin, rootfs, argv...)
	r.Cmd.Env = append(os.Environ(), fmt.Sprintf("TMPDIR=%s", r.TmpDir))

	for i, arg := range r.Cmd.Args {
		if arg == "--debug-bind-ip" {
			r.DebugIp = r.Cmd.Args[i+1]
		}
		if arg == "--debug-bind-port" {
			r.DebugPort, _ = strconv.Atoi(r.Cmd.Args[i+1])
		}
	}

	r.Runner = ginkgomon.New(ginkgomon.Config{
		Name:              "guardian",
		Command:           r.Cmd,
		AnsiColorCode:     "31m",
		StartCheck:        "guardian.started",
		StartCheckTimeout: 30 * time.Second,
	})

	return r
}

func Start(bin, initBin, nstarBin, dadooBin, grootfsBin, rootfs, tarBin string, argv ...string) *RunningGarden {
	runner := NewGardenRunner(bin, initBin, nstarBin, dadooBin, grootfsBin, rootfs, tarBin, "unix", fmt.Sprintf("/tmp/garden_%d.sock", GinkgoParallelNode()), argv...)

	r := &RunningGarden{
		runner:   runner,
		DepotDir: runner.DepotDir,

		DataDir:   DataDir,
		GraphPath: runner.GraphPath,
		Tmpdir:    runner.TmpDir,
		logger:    lagertest.NewTestLogger("garden-runner"),

		debugIP:   runner.DebugIp,
		debugPort: runner.DebugPort,

		Client: client.New(connection.New(runner.Network, runner.Addr)),
	}

	r.process = ifrit.Invoke(r.runner)
	r.Pid = runner.Cmd.Process.Pid

	return r
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

func (r *RunningGarden) DestroyAndStop() error {
	if err := r.DestroyContainers(); err != nil {
		return err
	}

	if err := r.Stop(); err != nil {
		return err
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

func cmd(tmpdir, depotDir, graphPath, network, addr, bin, initBin, nstarBin, dadooBin, grootfsBin, tarBin, rootfs string, argv ...string) *exec.Cmd {
	Expect(os.MkdirAll(tmpdir, 0755)).To(Succeed())

	snapshotsPath := filepath.Join(tmpdir, "snapshots")

	Expect(os.MkdirAll(depotDir, 0755)).To(Succeed())

	Expect(os.MkdirAll(snapshotsPath, 0755)).To(Succeed())

	appendDefaultFlag := func(ar []string, key, value string) []string {
		for _, a := range argv {
			if a == key {
				return ar
			}
		}

		if value != "" {
			return append(ar, key, value)
		} else {
			return append(ar, key)
		}
	}

	gardenArgs := make([]string, len(argv))
	copy(gardenArgs, argv)

	switch network {
	case "tcp":
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-ip", strings.Split(addr, ":")[0])
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-port", strings.Split(addr, ":")[1])
	case "unix":
		gardenArgs = appendDefaultFlag(gardenArgs, "--bind-socket", addr)
	}

	if rootfs != "" {
		gardenArgs = appendDefaultFlag(gardenArgs, "--default-rootfs", rootfs)
	}

	gardenArgs = appendDefaultFlag(gardenArgs, "--depot", depotDir)
	gardenArgs = appendDefaultFlag(gardenArgs, "--graph", graphPath)
	gardenArgs = appendDefaultFlag(gardenArgs, "--tag", fmt.Sprintf("%d", GinkgoParallelNode()))
	gardenArgs = appendDefaultFlag(gardenArgs, "--network-pool", fmt.Sprintf("10.254.%d.0/22", 4*GinkgoParallelNode()))
	gardenArgs = appendDefaultFlag(gardenArgs, "--init-bin", initBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--dadoo-bin", dadooBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--nstar-bin", nstarBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--tar-bin", tarBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--port-pool-start", fmt.Sprintf("%d", GinkgoParallelNode()*10000))
	gardenArgs = appendDefaultFlag(gardenArgs, "--debug-bind-ip", "0.0.0.0")
	gardenArgs = appendDefaultFlag(gardenArgs, "--debug-bind-port", fmt.Sprintf("%d", 8080+ginkgo.GinkgoParallelNode()))

	return exec.Command(bin, gardenArgs...)
}

func (r *RunningGarden) Cleanup() {
	// unmount aufs since the docker graph driver leaves this around,
	// otherwise the following commands might fail
	retry := retrier.New(retrier.ConstantBackoff(200, 500*time.Millisecond), nil)

	err := retry.Run(func() error {
		if err := os.RemoveAll(path.Join(r.GraphPath, "aufs")); err == nil {
			return nil // if we can remove it, it's already unmounted
		}

		if err := syscall.Unmount(path.Join(r.GraphPath, "aufs"), MNT_DETACH); err != nil {
			r.logger.Error("failed-unmount-attempt", err)
			return err
		}

		return nil
	})

	if err != nil {
		r.logger.Error("failed-to-unmount", err)
	}

	MustUnmountTmpfs(r.GraphPath)

	// In the kernel version 3.19.0-51-generic the code bellow results in
	// hanging the running VM. We are not deleting the node-X directories. They
	// are empty and the next test will re-use them. We will stick with that
	// workaround until we can test on a newer kernel that will hopefully not
	// have this bug.
	//
	// if err := os.RemoveAll(r.GraphPath); err != nil {
	// 	r.logger.Error("remove-graph", err)
	// }

	r.logger.Info("cleanup-tempdirs")
	if err := os.RemoveAll(r.Tmpdir); err != nil {
		r.logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.Tmpdir})
	} else {
		r.logger.Info("tempdirs-removed")
	}
}

func (r *RunningGarden) DestroyContainers() error {
	containers, err := r.Containers(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		r.Destroy(container.Handle())
	}

	return nil
}

type debugVars struct {
	NumGoRoutines int `json:"numGoRoutines"`
}

func (r *RunningGarden) NumGoroutines() (int, error) {
	debugURL := fmt.Sprintf("http://%s:%d/debug/vars", r.debugIP, r.debugPort)
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

func (r *RunningGarden) Buffer() *gbytes.Buffer {
	return r.runner.Buffer()
}

func (r *RunningGarden) ExitCode() int {
	return r.runner.ExitCode()
}

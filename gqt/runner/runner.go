package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var RootFSPath = os.Getenv("GARDEN_TEST_ROOTFS")
var GraphRoot = os.Getenv("GARDEN_TEST_GRAPHPATH")

type RunningGarden struct {
	client.Client
	process ifrit.Process

	Pid int

	tmpdir string

	DepotDir  string
	GraphRoot string
	GraphPath string

	logger lager.Logger
}

func Start(bin string, iodaemonBin string, argv ...string) *RunningGarden {
	network := "unix"
	addr := fmt.Sprintf("/tmp/garden_%d.sock", GinkgoParallelNode())
	tmpDir := filepath.Join(
		os.TempDir(),
		fmt.Sprintf("test-garden-%d", ginkgo.GinkgoParallelNode()),
	)

	if GraphRoot == "" {
		GraphRoot = filepath.Join(tmpDir, "graph")
	}

	graphPath := filepath.Join(GraphRoot, fmt.Sprintf("node-%d", ginkgo.GinkgoParallelNode()))
	depotDir := filepath.Join(tmpDir, "containers")

	r := &RunningGarden{
		DepotDir: depotDir,

		GraphRoot: GraphRoot,
		GraphPath: graphPath,
		tmpdir:    tmpDir,
		logger:    lagertest.NewTestLogger("garden-runner"),

		Client: client.New(connection.New(network, addr)),
	}

	c := cmd(tmpDir, depotDir, graphPath, network, addr, bin, iodaemonBin, RootFSPath, argv...)
	r.process = ifrit.Invoke(&ginkgomon.Runner{
		Name:              "guardian",
		Command:           c,
		AnsiColorCode:     "31m",
		StartCheck:        "guardian.started",
		StartCheckTimeout: 30 * time.Second,
	})

	r.Pid = c.Process.Pid

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
	select {
	case err := <-r.process.Wait():
		return err
	case <-time.After(time.Second * 10):
		r.process.Signal(syscall.SIGKILL)
		return errors.New("timed out waiting for garden to shutdown after 10 seconds")
	}
}

func cmd(tmpdir, depotDir, graphPath, network, addr, bin, iodaemonBin, rootFSPath string, argv ...string) *exec.Cmd {
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

	gardenArgs = appendDefaultFlag(gardenArgs, "--listenNetwork", network)
	gardenArgs = appendDefaultFlag(gardenArgs, "--listenAddr", addr)
	gardenArgs = appendDefaultFlag(gardenArgs, "--depot", depotDir)
	gardenArgs = appendDefaultFlag(gardenArgs, "--tag", fmt.Sprintf("%d", GinkgoParallelNode()))
	gardenArgs = appendDefaultFlag(gardenArgs, "--iodaemonBin", iodaemonBin)
	gardenArgs = appendDefaultFlag(gardenArgs, "--logLevel", "debug")
	gardenArgs = appendDefaultFlag(gardenArgs, "--debugAddr", fmt.Sprintf(":808%d", ginkgo.GinkgoParallelNode()))
	gardenArgs = appendDefaultFlag(gardenArgs, "--rootfs", rootFSPath)
	return exec.Command(bin, gardenArgs...)
}

func (r *RunningGarden) Cleanup() {
	if err := os.RemoveAll(r.GraphPath); err != nil {
		r.logger.Error("remove graph", err)
	}

	if os.Getenv("BTRFS_SUPPORTED") != "" {
		r.cleanupSubvolumes()
	}

	r.logger.Info("cleanup-tempdirs")
	if err := os.RemoveAll(r.tmpdir); err != nil {
		r.logger.Error("cleanup-tempdirs-failed", err, lager.Data{"tmpdir": r.tmpdir})
	} else {
		r.logger.Info("tempdirs-removed")
	}
}

func (r *RunningGarden) cleanupSubvolumes() {
	r.logger.Info("cleanup-subvolumes")

	// need to remove subvolumes before cleaning graphpath
	subvolumesOutput, err := exec.Command("btrfs", "subvolume", "list", "-o", r.GraphRoot).CombinedOutput()
	r.logger.Debug(fmt.Sprintf("listing-subvolumes: %s", string(subvolumesOutput)))
	if err != nil {
		r.logger.Fatal("listing-subvolumes-error", err)
	}

	for _, line := range strings.Split(string(subvolumesOutput), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		subvolumePath := fields[len(fields)-1] // this path is relative to the outer Garden-Linux BTRFS mount
		idx := strings.Index(subvolumePath, r.GraphRoot)
		if idx == -1 {
			continue
		}
		subvolumeAbsolutePath := subvolumePath[idx:]

		if strings.Contains(subvolumeAbsolutePath, r.GraphPath) {
			if b, err := exec.Command("btrfs", "subvolume", "delete", subvolumeAbsolutePath).CombinedOutput(); err != nil {
				r.logger.Fatal(fmt.Sprintf("deleting-subvolume: %s", string(b)), err)
			}
		}
	}

	if err := os.RemoveAll(r.GraphPath); err != nil {
		r.logger.Error("remove-graph-again", err)
	}
}

func (r *RunningGarden) DestroyContainers() error {
	containers, err := r.Containers(nil)
	if err != nil {
		return err
	}

	for _, container := range containers {
		err := r.Destroy(container.Handle())
		if err != nil {
			return err
		}
	}

	return nil
}

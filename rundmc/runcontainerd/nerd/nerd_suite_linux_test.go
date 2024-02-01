package nerd_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/lager/v3/lagertest"
	"github.com/BurntSushi/toml"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/plugins"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

type TestConfig struct {
	RunDir string
	Socket string
	CtrBin string
}

var (
	cgroupsPath string

	testConfig        *TestConfig
	containerdClient  *containerd.Client
	containerdContext context.Context
	containerdProcess *os.Process
)

func TestNerd(t *testing.T) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		if strings.Contains(message, "<requesting dmesg>") {
			GinkgoWriter.Write([]byte(fmt.Sprintf("Current UTC time is %s\n", time.Now().UTC().Format(time.RFC3339))))
			dmesgOutput, dmesgErr := exec.Command("/bin/dmesg", "-T").Output()
			if dmesgErr != nil {
				GinkgoWriter.Write([]byte(dmesgErr.Error()))
			}
			GinkgoWriter.Write(dmesgOutput)
		}

		Fail(message, callerSkip...)
	})
	RunSpecs(t, "Nerd Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	cgroupsPath = filepath.Join(os.TempDir(), "cgroups")
	setupCgroups(cgroupsPath)
	return nil
}, func(_ []byte) {})

var _ = BeforeEach(func() {
	if !isContainerd() {
		Skip("containerd not enabled")
	}

	runDir, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())

	containerdConfig := containerdrunner.ContainerdConfig(runDir)
	containerdProcess = containerdrunner.NewContainerdProcess(runDir, containerdConfig)

	containerdClient, err = containerd.New(containerdConfig.GRPC.Address, containerd.WithDefaultRuntime(plugin.RuntimeRuncV2))
	Expect(err).NotTo(HaveOccurred())

	containerdContext = namespaces.WithNamespace(context.Background(), fmt.Sprintf("nerdspace%d", GinkgoParallelProcess()))
	containerdContext = leases.WithLease(containerdContext, "lease-is-off-for-testing")

	testConfig = &TestConfig{
		RunDir: runDir,
		Socket: containerdConfig.GRPC.Address,
		CtrBin: "ctr",
	}
})

var _ = AfterEach(func() {
	if containerdProcess != nil {
		Expect(containerdProcess.Signal(syscall.SIGTERM)).To(Succeed())
		waitStatus, err := containerdProcess.Wait()
		Expect(err).NotTo(HaveOccurred())
		Expect(waitStatus.ExitCode()).To(BeZero())
	}
	Expect(os.RemoveAll(testConfig.RunDir)).To(Succeed())
	gexec.CleanupBuildArtifacts()
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	teardownCgroups(cgroupsPath)
})

func setupCgroups(cgroupsRoot string) {
	logger := lagertest.NewTestLogger("test")

	starter := cgroups.NewStarter(
		logger,
		mustOpen("/proc/cgroups"),
		mustOpen("/proc/self/cgroup"),
		cgroupsRoot,
		"nerd",
		[]specs.LinuxDeviceCgroup{},
		rundmc.IsMountPoint,
		false,
	)

	Expect(starter.Start()).To(Succeed())
}

func mustOpen(path string) *os.File {
	r, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())

	return r
}

func teardownCgroups(cgroupsRoot string) {
	mountsFileContent, err := ioutil.ReadFile("/proc/self/mounts")
	Expect(err).NotTo(HaveOccurred())

	lines := strings.Split(string(mountsFileContent), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if fields[2] == "cgroup" {
			Expect(unix.Unmount(fields[1], 0)).To(Succeed())
		}
	}

	Expect(unix.Unmount(cgroupsRoot, 0)).To(Succeed())
	Expect(os.Remove(cgroupsRoot)).To(Succeed())
}

func mustGetEnv(env string) string {
	if value := os.Getenv(env); value != "" {
		return value
	}
	panic(fmt.Sprintf("%s env must be non-empty", env))
}

func runCommandInDir(cmd *exec.Cmd, workingDir string) string {
	cmd.Dir = workingDir
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) string {
	var stdout bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Run()).To(Succeed())
	return stdout.String()
}

func jsonMarshal(v interface{}) []byte {
	buf := bytes.NewBuffer([]byte{})
	Expect(toml.NewEncoder(buf).Encode(v)).To(Succeed())
	return buf.Bytes()
}

func jsonUnmarshal(data []byte, v interface{}) {
	Expect(toml.Unmarshal(data, v)).To(Succeed())
}

func isContainerd() bool {
	return os.Getenv("CONTAINERD_ENABLED") == "true"
}

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
	"testing"

	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/burntsushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	. "github.com/onsi/ginkgo"
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

	containerdSession *gexec.Session
)

func TestNerd(t *testing.T) {
	RegisterFailHandler(Fail)
	SynchronizedBeforeSuite(func() []byte {
		cgroupsPath = filepath.Join(os.TempDir(), "cgroups")
		setupCgroups(cgroupsPath)

		runDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		containerdConfig := containerdrunner.ContainerdConfig(runDir)
		containerdSession = containerdrunner.NewSession(runDir, containerdConfig)

		testConfig := &TestConfig{
			RunDir: runDir,
			Socket: containerdConfig.GRPC.Address,
			CtrBin: "ctr",
		}
		return jsonMarshal(testConfig)
	}, func(data []byte) {
		testConfig = &TestConfig{}
		jsonUnmarshal(data, testConfig)

		var err error
		containerdClient, err = containerd.New(testConfig.Socket)
		Expect(err).NotTo(HaveOccurred())

		containerdContext = namespaces.WithNamespace(context.Background(), fmt.Sprintf("nerdspace%d", GinkgoParallelNode()))
	})

	SynchronizedAfterSuite(func() {}, func() {
		Expect(containerdSession.Terminate().Wait()).To(gexec.Exit(0))
		Expect(os.RemoveAll(testConfig.RunDir)).To(Succeed())
		teardownCgroups(cgroupsPath)
		gexec.CleanupBuildArtifacts()
	})

	RunSpecs(t, "Nerd Suite")
}

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

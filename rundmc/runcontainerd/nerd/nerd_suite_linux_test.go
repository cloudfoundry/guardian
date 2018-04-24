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
	"testing"

	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"github.com/burntsushi/toml"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

type TestConfig struct {
	RunDir string
	Socket string
	CtrBin string
}

var (
	testConfig        *TestConfig
	containerdClient  *containerd.Client
	containerdContext context.Context

	containerdSession *gexec.Session
)

func TestNerd(t *testing.T) {
	RegisterFailHandler(Fail)
	SynchronizedBeforeSuite(func() []byte {
		gdnBin := goCompile("code.cloudfoundry.org/guardian/cmd/gdn", "-tags", "daemon", "-ldflags", "-extldflags '-static'")
		setupCgroups(gdnBin)

		bins := getContainerdBinaries()

		runDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		containerdConfig := containerdrunner.ContainerdConfig(runDir)
		containerdSession = containerdrunner.NewSession(runDir, bins, containerdConfig)

		return jsonMarshal(&TestConfig{RunDir: runDir, Socket: containerdConfig.GRPC.Address, CtrBin: bins.Ctr})
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
		gexec.CleanupBuildArtifacts()
	})

	RunSpecs(t, "Nerd Suite")
}

func goCompile(mainPackagePath string, buildArgs ...string) string {
	if os.Getenv("RACE_DETECTION") != "" {
		buildArgs = append(buildArgs, "-race")
	}
	bin, err := gexec.Build(mainPackagePath, buildArgs...)
	Expect(err).NotTo(HaveOccurred())
	return bin
}

func setupCgroups(gdn string) {
	tag := "nerd-tests"
	tmpDir := filepath.Join(os.TempDir(), tag)

	cmd := exec.Command(gdn, "setup", "--tag", tag)
	cmd.Env = append(
		[]string{
			fmt.Sprintf("TMPDIR=%s", tmpDir),
			fmt.Sprintf("TEMP=%s", tmpDir),
			fmt.Sprintf("TMP=%s", tmpDir),
		},
		os.Environ()...,
	)
	setupProcess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	Expect(setupProcess.Wait().ExitCode()).To(BeZero())
}

func getContainerdBinaries() containerdrunner.Binaries {
	containerdBin := makeContainerd()
	return containerdrunner.Binaries{
		Dir:        containerdBin,
		Containerd: filepath.Join(containerdBin, "containerd"),
		Ctr:        filepath.Join(containerdBin, "ctr"),
	}
}

func makeContainerd() string {
	containerdPath := filepath.Join(mustGetEnv("GOPATH"), filepath.FromSlash("src/github.com/containerd/containerd"))
	makeContainerdCommand := exec.Command("make")
	makeContainerdCommand.Env = append(os.Environ(), "BUILDTAGS=no_btrfs")
	runCommandInDir(makeContainerdCommand, containerdPath)
	return filepath.Join(containerdPath, "bin")
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

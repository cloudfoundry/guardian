package gqt_cleanup_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"github.com/BurntSushi/toml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	binaries          runner.Binaries
	config            runner.GdnRunnerConfig
	defaultTestRootFS string

	containerdProcess *os.Process
	containerdRunDir  string
)

func TestSetupGqt(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Cleanup Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	binaries := getGardenBinaries()

	// chmod all the artifacts
	Expect(os.Chmod(filepath.Join(binaries.Gdn, "..", ".."), 0755)).To(Succeed())
	filepath.Walk(filepath.Join(binaries.Gdn, "..", ".."), func(path string, info os.FileInfo, err error) error {
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(path, 0755)).To(Succeed())
		return nil
	})

	return jsonMarshal(binaries)
}, func(data []byte) {
	bins := new(runner.Binaries)
	jsonUnmarshal(data, bins)
	binaries = *bins
	defaultTestRootFS = os.Getenv("GARDEN_TEST_ROOTFS")
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	if defaultTestRootFS == "" {
		Skip("No Garden RootFS")
	}
	config = defaultConfig()
	if isContainerd() {
		containerdRunDir = tempDir("", "")
		containerdProcess = startContainerd(containerdRunDir)
	}
})

var _ = AfterEach(func() {
	if isContainerd() {
		terminateContainerd()
		Expect(os.RemoveAll(containerdRunDir)).To(Succeed())
	}

	Expect(os.RemoveAll(config.TmpDir)).To(Succeed())
})

func terminateContainerd() {
	if err := containerdProcess.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(GinkgoWriter, "WARNING: Failed to kill containerd process: %v", err)
	}
	waitStatus, err := containerdProcess.Wait()
	Expect(err).NotTo(HaveOccurred())
	Expect(waitStatus.ExitCode()).To(BeZero())
}

func defaultConfig() runner.GdnRunnerConfig {
	cfg := runner.DefaultGdnRunnerConfig(binaries)
	cfg.DefaultRootFS = defaultTestRootFS
	cfg.GdnBin = binaries.Gdn
	cfg.GrootBin = binaries.Groot
	cfg.Socket2meBin = binaries.Socket2me
	cfg.ExecRunnerBin = binaries.ExecRunner
	cfg.InitBin = binaries.Init
	cfg.TarBin = binaries.Tar
	cfg.NSTarBin = binaries.NSTar
	cfg.ImagePluginBin = binaries.Groot
	cfg.PrivilegedImagePluginBin = binaries.Groot

	return cfg
}

func compileGdn(additionalCompileArgs ...string) string {
	defaultCompileArgs := []string{"-tags", "daemon"}
	compileArgs := append(defaultCompileArgs, additionalCompileArgs...)

	return goCompile("code.cloudfoundry.org/guardian/cmd/gdn", compileArgs...)
}

func goCompile(mainPackagePath string, buildArgs ...string) string {
	if os.Getenv("RACE_DETECTION") != "" {
		buildArgs = append(buildArgs, "-race")
	}
	buildArgs = append(buildArgs, "-mod=vendor")
	bin, err := gexec.Build(mainPackagePath, buildArgs...)
	Expect(err).NotTo(HaveOccurred())
	return bin
}

func getGardenBinaries() runner.Binaries {
	gardenBinaries := runner.Binaries{
		Gdn: compileGdn(),
	}

	if runtime.GOOS == "linux" {
		gardenBinaries.ExecRunner = os.Getenv("DADOO_BINARY")
		gardenBinaries.Socket2me = os.Getenv("SOCKET2ME_BINARY")
		gardenBinaries.NSTar = os.Getenv("NSTAR_BINARY")
		gardenBinaries.Init = os.Getenv("INIT_BINARY")
		gardenBinaries.Groot = os.Getenv("GROOTFS_BINARY")
		gardenBinaries.Tardis = os.Getenv("GROOTFS_TARDIS_BINARY")
	}

	return gardenBinaries
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

func startContainerd(runDir string) *os.Process {
	containerdConfig := containerdrunner.ContainerdConfig(runDir)
	config.ContainerdSocket = containerdConfig.GRPC.Address
	return containerdrunner.NewContainerdProcess(runDir, containerdConfig)
}

func tempDir(dir, prefix string) string {
	path, err := ioutil.TempDir(dir, prefix)
	Expect(err).NotTo(HaveOccurred())
	return path
}

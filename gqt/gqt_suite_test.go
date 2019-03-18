package gqt_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"code.cloudfoundry.org/guardian/gqt/helpers"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/pkg/locksmith"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

const (
	kb uint64 = 1024
	mb        = 1024 * kb
)

var (
	ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}
	// the unprivileged user is baked into the cfgarden/garden-ci-ubuntu image
	unprivilegedUID = uint32(5000)
	unprivilegedGID = uint32(5000)

	containerdConfig  containerdrunner.Config
	containerdSession *gexec.Session
	containerdRunDir  string

	config            runner.GdnRunnerConfig
	binaries          runner.Binaries
	defaultTestRootFS string

	buildNoGExecDir string
)

func goCompile(mainPackagePath string, buildArgs ...string) string {
	if os.Getenv("RACE_DETECTION") != "" {
		buildArgs = append(buildArgs, "-race")
	}
	bin, err := gexec.Build(mainPackagePath, buildArgs...)
	Expect(err).NotTo(HaveOccurred())
	return bin
}

type runnerBinaries struct {
	Garden runner.Binaries
}

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	binaries := runnerBinaries{
		Garden: helpers.GetGardenBinaries(),
	}

	// chmod all the artifacts
	Expect(os.Chmod(filepath.Join(binaries.Garden.Gdn, "..", ".."), 0755)).To(Succeed())
	filepath.Walk(filepath.Join(binaries.Garden.Gdn, "..", ".."), func(path string, info os.FileInfo, err error) error {
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(path, 0755)).To(Succeed())
		return nil
	})

	return helpers.JsonMarshal(binaries)
}, func(data []byte) {
	bins := new(runnerBinaries)
	helpers.JsonUnmarshal(data, bins)
	binaries = bins.Garden
	defaultTestRootFS = os.Getenv("GARDEN_TEST_ROOTFS")
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	if defaultTestRootFS == "" {
		Skip("No Garden RootFS")
	}

	config = helpers.DefaultConfig(defaultTestRootFS, binaries)
	if isContainerd() {
		containerdRunDir = tempDir("", "")
		containerdSession = startContainerd(containerdRunDir)
	}

})

var _ = AfterEach(func() {
	if isContainerd() {
		Expect(containerdSession.Terminate().Wait()).To(gexec.Exit(0))
		Expect(os.RemoveAll(containerdRunDir)).To(Succeed())
	}

	// Windows worker is not containerised and therefore the test needs to take care to delete the temporary folder
	if runtime.GOOS == "windows" {
		Expect(os.RemoveAll(config.TmpDir)).To(Succeed())
	}
})

func runCommand(cmd *exec.Cmd) string {
	return helpers.RunCommandInDir(cmd, "")
}

func restartGarden(client *runner.RunningGarden, config runner.GdnRunnerConfig) *runner.RunningGarden {
	Expect(client.Ping()).To(Succeed(), "tried to restart garden while it was not running")
	Expect(client.Stop()).To(Succeed())
	return runner.Start(config)
}

func runIPTables(ipTablesArgs ...string) ([]byte, error) {
	lock, err := locksmith.NewFileSystem().Lock(iptables.LockKey)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()

	outBuffer := bytes.NewBuffer([]byte{})
	errBuffer := bytes.NewBuffer([]byte{})
	cmd := exec.Command("iptables", append([]string{"-w"}, ipTablesArgs...)...)
	cmd.Stdout = outBuffer
	cmd.Stderr = errBuffer
	err = cmd.Run()

	fmt.Fprintln(GinkgoWriter, outBuffer.String())
	fmt.Fprintln(GinkgoWriter, errBuffer.String())
	return outBuffer.Bytes(), err
}

func intptr(i int) *int {
	return &i
}

func uint64ptr(i uint64) *uint64 {
	return &i
}

func uint32ptr(i uint32) *uint32 {
	return &i
}

func boolptr(b bool) *bool {
	return &b
}

func stringptr(s string) *string {
	return &s
}

func idToStr(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}

func readFile(path string) []byte {
	return helpers.ReadFile(path)
}

func readFileString(path string) string {
	return string(readFile(path))
}

func tempDir(dir, prefix string) string {
	path, err := ioutil.TempDir(dir, prefix)
	Expect(err).NotTo(HaveOccurred())
	return path
}

func tempFile(dir, prefix string) *os.File {
	f, err := ioutil.TempFile(dir, prefix)
	Expect(err).NotTo(HaveOccurred())
	return f
}

func copyFile(srcPath, dstPath string) error {
	dirPath := filepath.Dir(dstPath)
	if err := os.MkdirAll(dirPath, 0777); err != nil {
		return err
	}

	reader, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	writer, err := os.Create(dstPath)
	if err != nil {
		reader.Close()
		return err
	}

	if _, err := io.Copy(writer, reader); err != nil {
		writer.Close()
		reader.Close()
		return err
	}

	writer.Close()
	reader.Close()

	return os.Chmod(writer.Name(), 0777)
}

func removeSocket() {
	_, err := os.Stat(config.BindSocket)
	if err == nil {
		Expect(os.Remove(config.BindSocket)).To(Succeed())
	} else if !os.IsNotExist(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}

func createPeaRootfs() string {
	return createRootfs(func(root string) {
		Expect(exec.Command("chown", "-R", "4294967294:4294967294", root).Run()).To(Succeed())
		Expect(ioutil.WriteFile(filepath.Join(root, "ima-pea"), []byte("pea!"), 0644)).To(Succeed())
	}, 0777)
}

func createPeaRootfsTar() string {
	return tarUpDir(createPeaRootfs())
}

func createRootfsTar(modifyRootfs func(string)) string {
	return tarUpDir(createRootfs(modifyRootfs, 0755))
}

func createRootfs(modifyRootfs func(string), perm os.FileMode) string {
	tmpDir := tempDir("", "test-rootfs")
	unpackedRootfs := filepath.Join(tmpDir, "unpacked")
	Expect(os.Mkdir(unpackedRootfs, perm)).To(Succeed())
	runCommand(exec.Command("tar", "xf", defaultTestRootFS, "-C", unpackedRootfs))

	Expect(os.Chmod(tmpDir, perm)).To(Succeed())
	modifyRootfs(unpackedRootfs)

	return unpackedRootfs
}

func tarUpDir(path string) string {
	tarPath := filepath.Join(filepath.Dir(path), filepath.Base(path)+".tar")
	repackCmd := exec.Command("sh", "-c", fmt.Sprintf("tar cf %s *", tarPath))
	helpers.RunCommandInDir(repackCmd, path)

	return tarPath
}

func resetImagePluginConfig() runner.GdnRunnerConfig {
	config.ImagePluginBin = ""
	config.PrivilegedImagePluginBin = ""
	config.ImagePluginExtraArgs = []string{}
	config.PrivilegedImagePluginExtraArgs = []string{}
	return config
}

func mustGetEnv(env string) string {
	if value := os.Getenv(env); value != "" {
		return value
	}
	panic(fmt.Sprintf("%s env must be non-empty", env))
}

func isContainerd() bool {
	return os.Getenv("CONTAINERD_ENABLED") == "true"
}

func skipIfContainerd() {
	if isContainerd() {
		Skip("irrelevant test for containerd mode")
	}
}

func skipIfNotContainerd() {
	if !isContainerd() {
		Skip("containerd not enabled")
	}
}

func skipIfDev() {
	if os.Getenv("LOCAL_DEV_RUN") == "true" {
		Skip("skipping when running locally")
	}
}

func getRuncRoot() string {
	if config.ContainerdSocket != "" {
		return "/run/containerd/runc/garden"
	}

	return "/run/runc"
}

func startContainerd(runDir string) *gexec.Session {
	containerdConfig := containerdrunner.ContainerdConfig(runDir)
	config.ContainerdSocket = containerdConfig.GRPC.Address
	return containerdrunner.NewSession(runDir, containerdConfig)
}

func restartContainerd() {
	containerdSession.Terminate().Wait()
	containerdSession = startContainerd(containerdRunDir)
}

func numGoRoutines(client *runner.RunningGarden) int {
	numGoroutines, err := client.NumGoroutines()
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return numGoroutines
}

func pollNumGoRoutines(client *runner.RunningGarden) func() int {
	return func() int {
		return numGoRoutines(client)
	}
}

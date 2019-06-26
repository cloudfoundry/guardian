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
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/containerdrunner"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/pkg/locksmith"
	"github.com/BurntSushi/toml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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
	buildArgs = append(buildArgs, "-mod=vendor")
	bin, err := gexec.Build(mainPackagePath, buildArgs...)
	Expect(err).NotTo(HaveOccurred())
	return bin
}

type runnerBinaries struct {
	Garden runner.Binaries
}

func TestGqt(t *testing.T) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		if strings.Contains(message, "<requesting dmesg>") {
			GinkgoWriter.Write([]byte(fmt.Sprintf("Current UTC time is %s\n", time.Now().UTC().Format(time.RFC3339))))
			dmesgOutput, dmesgErr := exec.Command("/bin/dmesg", "-T").Output()
			if dmesgErr != nil {
				GinkgoWriter.Write([]byte(dmesgErr.Error()))
			}
			GinkgoWriter.Write(dmesgOutput)
		}

		if strings.Contains(message, "container init still running") {
			GinkgoWriter.Write([]byte(fmt.Sprintf("Current Ginkgo node is %d\n", GinkgoParallelNode())))
			psTreeOut, psTreeErr := exec.Command("ps", "auxf").Output()
			if psTreeErr != nil {
				GinkgoWriter.Write([]byte(psTreeErr.Error()))
			}
			GinkgoWriter.Write(psTreeOut)

			dstatedOut, dstatedErr := exec.Command("sh", "-c", `ps -eLo pid,tid,ppid,user:11,comm,state,wchan | grep "D "`).Output()
			if dstatedErr != nil {
				GinkgoWriter.Write([]byte(dstatedErr.Error()))
			}
			GinkgoWriter.Write(dstatedOut)
		}

		if strings.Contains(message, "running image plugin destroy: deleting image path") {
			GinkgoWriter.Write([]byte(fmt.Sprintf("Current Ginkgo node is %d\n", GinkgoParallelNode())))
			GinkgoWriter.Write([]byte("Printing rootfs directories inodes...\n\n"))
			findOut, findErr := exec.Command("/bin/bash", "-c", fmt.Sprintf("find %s -iname 'rootfs' -printf '%%p %%i\n'", config.StorePath)).Output()
			if findErr != nil {
				GinkgoWriter.Write([]byte(findErr.Error()))
			}
			GinkgoWriter.Write(findOut)

			GinkgoWriter.Write([]byte("Printing lsof...\n\n"))
			lsofOut, lsofErr := exec.Command("lsof").Output()
			if lsofErr != nil {
				GinkgoWriter.Write([]byte(lsofErr.Error()))
			}
			GinkgoWriter.Write(lsofOut)
		}

		Fail(message, callerSkip...)
	})
	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	binaries := runnerBinaries{
		Garden: getGardenBinaries(),
	}

	// chmod all the artifacts
	Expect(os.Chmod(filepath.Join(binaries.Garden.Gdn, "..", ".."), 0755)).To(Succeed())
	filepath.Walk(filepath.Join(binaries.Garden.Gdn, "..", ".."), func(path string, info os.FileInfo, err error) error {
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(path, 0755)).To(Succeed())
		return nil
	})

	return jsonMarshal(binaries)
}, func(data []byte) {
	bins := new(runnerBinaries)
	jsonUnmarshal(data, bins)
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

	config = defaultConfig()
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

func getGardenBinaries() runner.Binaries {
	gardenBinaries := runner.Binaries{
		Tar:           os.Getenv("GARDEN_TAR_PATH"),
		Gdn:           CompileGdn(),
		NetworkPlugin: goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_network_plugin"),
		ImagePlugin:   goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_image_plugin"),
		RuntimePlugin: goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_runtime_plugin"),
		NoopPlugin:    goCompile("code.cloudfoundry.org/guardian/gqt/cmd/noop_plugin"),
	}

	gardenBinaries.PrivilegedImagePlugin = gardenBinaries.ImagePlugin + "-priv"
	Expect(copyFile(gardenBinaries.ImagePlugin, gardenBinaries.PrivilegedImagePlugin)).To(Succeed())

	if runtime.GOOS == "linux" {
		gardenBinaries.ExecRunner = goCompile("code.cloudfoundry.org/guardian/cmd/dadoo")
		gardenBinaries.Socket2me = goCompile("code.cloudfoundry.org/guardian/cmd/socket2me")

		cmd := exec.Command("make")
		runCommandInDir(cmd, "../rundmc/nstar")
		gardenBinaries.NSTar = "../rundmc/nstar/nstar"

		cmd = exec.Command("gcc", "-static", "-o", "init", "init.c", "ignore_sigchild.c")
		runCommandInDir(cmd, "../cmd/init")
		gardenBinaries.Init = "../cmd/init/init"

		gardenBinaries.Groot = findInGoPathBin("grootfs")
		gardenBinaries.Tardis = findInGoPathBin("tardis")
	}

	return gardenBinaries
}

func findInGoPathBin(binary string) string {
	gopath, ok := os.LookupEnv("GOPATH")
	Expect(ok).To(BeTrue(), "GOPATH must be set")
	binPath := filepath.Join(gopath, "bin", binary)
	Expect(binPath).To(BeAnExistingFile(), fmt.Sprintf("%s does not exist", binPath))
	return binPath
}

func CompileGdn(additionalCompileArgs ...string) string {
	defaultCompileArgs := []string{"-tags", "daemon"}
	compileArgs := append(defaultCompileArgs, additionalCompileArgs...)

	return goCompile("code.cloudfoundry.org/guardian/cmd/gdn", compileArgs...)
}

func runCommandInDir(cmd *exec.Cmd, workingDir string) string {
	cmd.Dir = workingDir
	cmdOutput, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Running command %#v failed: %v: %s", cmd, err, string(cmdOutput)))
	return string(cmdOutput)
}

func runCommand(cmd *exec.Cmd) string {
	return runCommandInDir(cmd, "")
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

// returns the n'th ASCII character starting from 'a' through 'z'
// E.g. nodeToString(1) = a, nodeToString(2) = b, etc ...
func nodeToString(ginkgoNode int) string {
	r := 'a' + ginkgoNode - 1
	Expect(r).To(BeNumerically(">=", 'a'))
	Expect(r).To(BeNumerically("<=", 'z'))
	return string(r)
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
	content, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return content
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
	runCommandInDir(repackCmd, path)

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

func runInContainerWithIO(container garden.Container, processIO garden.ProcessIO, path string, args []string) {
	proc, err := container.Run(
		garden.ProcessSpec{
			Path: path,
			Args: args,
		},
		processIO)
	Expect(err).NotTo(HaveOccurred())

	exitCode, err := proc.Wait()
	Expect(err).NotTo(HaveOccurred())
	Expect(exitCode).To(Equal(0))
}

func runInContainer(container garden.Container, path string, args []string) {
	runInContainerWithIO(container, ginkgoIO, path, args)
}

func runInContainerCombinedOutput(container garden.Container, path string, args []string) string {
	output := gbytes.NewBuffer()
	pio := garden.ProcessIO{
		Stdout: output,
		Stderr: output,
	}
	runInContainerWithIO(container, pio, path, args)
	return string(output.Contents())
}

func listCgroups(pid string) string {
	cgroups, err := ioutil.ReadFile(filepath.Join("/proc", pid, "cgroup"))
	Expect(err).NotTo(HaveOccurred())
	return string(cgroups)
}

func getRuncContainerPID(handle string) string {
	pidBytes, err := ioutil.ReadFile(filepath.Join(config.DepotDir, handle, "pidfile"))
	Expect(err).NotTo(HaveOccurred())
	return string(pidBytes)
}

func getContainerdContainerPID(handle string) string {
	processes := listProcesses("ctr", config.ContainerdSocket, handle)
	return pidFromProcessesOutput(processes, handle)
}

func getContainerPid(handle string) string {
	if isContainerd() {
		return getContainerdContainerPID(handle)
	}
	return getRuncContainerPID(handle)
}

func listPidsInCgroup(cgroupPath string) string {
	content := readFileString(filepath.Join(cgroupPath, "cgroup.procs"))
	procs := strings.Split(content, " ")
	result := content + "\n"

	for _, p := range procs {
		p = strings.TrimSpace(p)
		result = result + fmt.Sprintf("cmdline for %s: %s", p, getCmdLine(p)) + "\n"
	}

	return result
}

func getCmdLine(pid string) string {
	cmdBytes, err := ioutil.ReadFile(filepath.Join("/proc", pid, "cmdline"))
	Expect(err).NotTo(HaveOccurred())
	return string(cmdBytes)
}

func ociBundlesDir() string {
	if isContainerd() {
		return filepath.Join(containerdRunDir, "state", "io.containerd.runtime.v1.linux", "garden")
	}
	return config.DepotDir
}

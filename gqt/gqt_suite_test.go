package gqt_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
	containerdProcess *os.Process
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
			io.WriteString(GinkgoWriter, fmt.Sprintf("\n\nCurrent UTC time is %s\n", time.Now().UTC().Format(time.RFC3339)))
			dmesgOutput, dmesgErr := exec.Command("/bin/dmesg", "-T").Output()
			if dmesgErr != nil {
				io.WriteString(GinkgoWriter, dmesgErr.Error())
			}
			GinkgoWriter.Write(dmesgOutput)
		}

		if strings.Contains(message, "running image plugin destroy: deleting image path") {
			io.WriteString(GinkgoWriter, fmt.Sprintf("\n\nCurrent Ginkgo node is %d\n", GinkgoParallelNode()))

			io.WriteString(GinkgoWriter, "\nPrinting the mount table...\n\n")
			mntTableOut, mntTableErr := getMountTable()
			if mntTableErr != nil {
				io.WriteString(GinkgoWriter, mntTableErr.Error())
			}
			io.WriteString(GinkgoWriter, mntTableOut)

			io.WriteString(GinkgoWriter, "\nPrinting the process tree...\n\n")
			psOut, psErr := exec.Command("ps", "auxf").Output()
			if psErr != nil {
				io.WriteString(GinkgoWriter, psErr.Error())
			}
			GinkgoWriter.Write(psOut)

			r, _ := regexp.Compile("deleting image path '(.+)' failed")
			submatches := r.FindStringSubmatch(message)
			if len(submatches) >= 2 {
				imagePath := submatches[1]
				rootfsPath := filepath.Join(imagePath, "rootfs")

				io.WriteString(GinkgoWriter, fmt.Sprintf("\nPrinting fuser on %s...\n\n", rootfsPath))
				fuserOut, fuserErr := exec.Command("fuser", "-m", rootfsPath).CombinedOutput()
				if fuserErr != nil {
					io.WriteString(GinkgoWriter, fuserErr.Error())
				}
				GinkgoWriter.Write(fuserOut)
				fmt.Fprintln(GinkgoWriter)

				Eventually(getMountTable, "2m", "1s").ShouldNot(ContainSubstring(rootfsPath), "Image rootfs path %q is still a mountpoint", rootfsPath)
			}
		}

		if strings.Contains(message, "failed getting task") {
			io.WriteString(GinkgoWriter, fmt.Sprintf("\n\nCurrent Ginkgo node is %d\n", GinkgoParallelNode()))

			io.WriteString(GinkgoWriter, "\nPrinting the containerd tasks...\n\n")
			io.WriteString(GinkgoWriter, listTasks("ctr", config.ContainerdSocket))

			io.WriteString(GinkgoWriter, "\nPrinting the process tree...\n\n")
			psOut, err := exec.Command("ps", "auxf").Output()
			if err != nil {
				io.WriteString(GinkgoWriter, err.Error())
			}
			GinkgoWriter.Write(psOut)

			io.WriteString(GinkgoWriter, "\nPrinting runc containers...\n\n")
			runcOut, err := exec.Command("runc", "--root", "/run/containerd/runc/garden", "list").Output()
			if err != nil {
				io.WriteString(GinkgoWriter, err.Error())
			}
			GinkgoWriter.Write(runcOut)

			pids, err := exec.Command("pidof", "containerd-shim").Output()
			if err != nil {
				io.WriteString(GinkgoWriter, err.Error())
			}

			io.WriteString(GinkgoWriter, "\nPrinting shim pids...\n\n")
			GinkgoWriter.Write(pids)

			for _, pid := range strings.Split(string(pids), " ") {
				io.WriteString(GinkgoWriter, fmt.Sprintf("\nPrinting shim stack trace for pid %s...\n\n", pid))
				stack, err := exec.Command("cat", fmt.Sprintf("/proc/%s/stack", strings.TrimSpace(pid))).Output()
				if err != nil {
					io.WriteString(GinkgoWriter, err.Error())
				}

				GinkgoWriter.Write(stack)
			}
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
		containerdProcess = startContainerd(containerdRunDir)
	}

	if cpuThrottlingEnabled() {
		config.EnableCPUThrottling = boolptr(true)
		config.CPUThrottlingCheckInterval = uint64ptr(5)
	}
})

var _ = AfterEach(func() {
	if isContainerd() {
		terminateContainerd()
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

func getGardenBinaries() runner.Binaries {
	gardenBinaries := runner.Binaries{
		Tar:           os.Getenv("GARDEN_TAR_PATH"),
		Gdn:           CompileGdn(),
		NetworkPlugin: goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_network_plugin"),
		ImagePlugin:   goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_image_plugin"),
		RuntimePlugin: goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_runtime_plugin"),
		NoopPlugin:    goCompile("code.cloudfoundry.org/guardian/gqt/cmd/noop_plugin"),
		FakeRunc:      goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_runc"),
		Mkdirer:       goCompile("code.cloudfoundry.org/guardian/cmd/mkdir"),
	}

	gardenBinaries.PrivilegedImagePlugin = gardenBinaries.ImagePlugin + "-priv"
	Expect(copyFile(gardenBinaries.ImagePlugin, gardenBinaries.PrivilegedImagePlugin)).To(Succeed())

	if runtime.GOOS == "linux" {
		gardenBinaries.ExecRunner = goCompile("code.cloudfoundry.org/guardian/cmd/dadoo")
		gardenBinaries.Socket2me = goCompile("code.cloudfoundry.org/guardian/cmd/socket2me")
		gardenBinaries.FakeRuncStderr = goCompile("code.cloudfoundry.org/guardian/gqt/cmd/fake_runc_stderr")

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

func runCommandInDir(cmd *exec.Cmd, workingDir string, diagnosticFunc ...func() string) string {
	cmd.Dir = workingDir
	cmdOutput, err := cmd.CombinedOutput()
	diagnostics := []interface{}{fmt.Sprintf("Running command %#v failed: %v: %s", cmd, err, string(cmdOutput))}

	for _, f := range diagnosticFunc {
		diagnostics = append(diagnostics, f())
	}
	Expect(err).NotTo(HaveOccurred(), diagnostics...)
	return string(cmdOutput)
}

func runCommand(cmd *exec.Cmd, diagnosticFunc ...func() string) string {
	return runCommandInDir(cmd, "", diagnosticFunc...)
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
	cfg.MkdirerBin = binaries.Mkdirer

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

func cpuThrottlingEnabled() bool {
	return os.Getenv("CPU_THROTTLING_ENABLED") == "true"
}

func isContainerd() bool {
	return os.Getenv("CONTAINERD_ENABLED") == "true"
}

func isContainerdForProcesses() bool {
	return os.Getenv("CONTAINERD_FOR_PROCESSES_ENABLED") == "true"
}

func skipIfContainerd() {
	if isContainerd() {
		Skip("irrelevant test for containerd mode")
	}
}

func skipIfContainerdForProcesses(reason string) {
	if isContainerdForProcesses() {
		Skip(reason)
	}
}

func skipIfRunDmcForProcesses(reason string) {
	if !isContainerdForProcesses() {
		Skip(reason)
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

func skipIfNotCPUThrottling() {
	if os.Getenv("CPU_THROTTLING_ENABLED") != "true" {
		Skip("skipping as CPU throttling is not enabled")
	}
}

func getRuncRoot() string {
	if config.ContainerdSocket != "" {
		return "/run/containerd/runc/garden"
	}

	return "/run/runc"
}

func startContainerd(runDir string) *os.Process {
	containerdConfig := containerdrunner.ContainerdConfig(runDir)
	config.ContainerdSocket = containerdConfig.GRPC.Address
	config.UseContainerdForProcesses = boolptr(isContainerdForProcesses())
	return containerdrunner.NewContainerdProcess(runDir, containerdConfig)
}

func restartContainerd() {
	terminateContainerd()
	containerdProcess = startContainerd(containerdRunDir)
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

func getMountTable() (string, error) {
	output, err := exec.Command("cat", "/proc/self/mountinfo").Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

package gqt_test

import (
	"bytes"
	"errors"
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
	"github.com/burntsushi/toml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}
	// the unprivileged user is baked into the cfgarden/garden-ci-ubuntu image
	unprivilegedUID = uint32(5000)
	unprivilegedGID = uint32(5000)

	config             runner.GdnRunnerConfig
	containerdConfig   containerdrunner.Config
	binaries           runner.Binaries
	containerdBinaries containerdrunner.Binaries
	gqtStartTime       time.Time
	defaultTestRootFS  string
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
	Garden     runner.Binaries
	Containerd containerdrunner.Binaries
}

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		return jsonMarshal(runnerBinaries{
			Garden:     getGardenBinaries(),
			Containerd: getContainerdBinaries(),
		})
	}, func(data []byte) {
		bins := new(runnerBinaries)
		jsonUnmarshal(data, bins)
		binaries = bins.Garden
		containerdBinaries = bins.Containerd
		defaultTestRootFS = os.Getenv("GARDEN_TEST_ROOTFS")
	})

	SynchronizedAfterSuite(func() {}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		if defaultTestRootFS == "" {
			Skip("No Garden RootFS")
		}

		// chmod all the artifacts
		Expect(os.Chmod(filepath.Join(binaries.Gdn, "..", ".."), 0755)).To(Succeed())
		filepath.Walk(filepath.Join(binaries.Gdn, "..", ".."), func(path string, info os.FileInfo, err error) error {
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(path, 0755)).To(Succeed())
			return nil
		})

		config = defaultConfig()
		if runtime.GOOS == "linux" {
			initGrootStore(config.ImagePluginBin, config.StorePath, []string{"0:4294967294:1", "1:65536:4294901758"})
			initGrootStore(config.PrivilegedImagePluginBin, config.PrivilegedStorePath, nil)
		}

		containerdConfig = defaultContainerdConfig()
	})

	AfterEach(func() {
		// Windows worker is not containerised and therefore the test needs to take care to delete the temporary folder
		if runtime.GOOS == "windows" {
			Expect(os.RemoveAll(config.TmpDir)).To(Succeed())
		}
	})

	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

func getGardenBinaries() runner.Binaries {
	gardenBinaries := runner.Binaries{
		Tar:           os.Getenv("GARDEN_TAR_PATH"),
		Gdn:           goCompile("code.cloudfoundry.org/guardian/cmd/gdn", "-tags", "daemon", "-ldflags", "-extldflags '-static'"),
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

		cmd = exec.Command("gcc", "-static", "-o", "init", "init.c")
		runCommandInDir(cmd, "../cmd/init")
		gardenBinaries.Init = "../cmd/init/init"

		gardenBinaries.Groot = goCompile("code.cloudfoundry.org/grootfs")
		gardenBinaries.Tardis = goCompile("code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis")
		Expect(os.Chmod(gardenBinaries.Tardis, 04755)).To(Succeed())
	}

	return gardenBinaries
}

func getContainerdBinaries() containerdrunner.Binaries {
	containerdBin := makeContainerd()

	return containerdrunner.Binaries{
		Containerd: filepath.Join(containerdBin, "containerd"),
		Ctr:        filepath.Join(containerdBin, "ctr"),
		Dir:        containerdBin,
	}
}

func initGrootStore(grootBin, storePath string, idMappings []string) {
	initStoreArgs := []string{"--store", storePath, "init-store", "--store-size-bytes", fmt.Sprintf("%d", 2*1024*1024*1024)}
	for _, idMapping := range idMappings {
		initStoreArgs = append(initStoreArgs, "--uid-mapping", idMapping, "--gid-mapping", idMapping)
	}

	initStore := exec.Command(grootBin, initStoreArgs...)
	initStore.Stdout = GinkgoWriter
	initStore.Stderr = GinkgoWriter
	Expect(initStore.Run()).To(Succeed())
}

func runCommandInDir(cmd *exec.Cmd, workingDir string) string {
	var stdout bytes.Buffer
	cmd.Dir = workingDir
	cmd.Stdout = io.MultiWriter(&stdout, GinkgoWriter)
	cmd.Stderr = GinkgoWriter
	Expect(cmd.Run()).To(Succeed())
	return stdout.String()
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

func defaultContainerdConfig() containerdrunner.Config {
	containerdDataDir, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())

	cfg := containerdrunner.ContainerdConfig(containerdDataDir)
	cfg.ContainerdBin = containerdBinaries.Containerd
	cfg.CtrBin = containerdBinaries.Ctr
	cfg.BinariesDir = containerdBinaries.Dir

	cfg.RunDir = containerdDataDir

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

func readFile(path string) string {
	content, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())
	return string(content)
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

func getCurrentCGroup(cgroupToFind string) (string, error) {
	cgroupContent, err := ioutil.ReadFile(fmt.Sprintf("/proc/self/cgroup"))
	if err != nil {
		return "", err
	}

	cgroups := strings.Split(string(cgroupContent), "\n")
	for _, cgroup := range cgroups {
		fields := strings.Split(cgroup, ":")
		if len(fields) != 3 {
			return "", errors.New("Error parsing cgroups:" + cgroup)
		}
		subsystems := strings.Split(fields[1], ",")
		if containsElement(subsystems, cgroupToFind) {
			return fields[2], nil
		}
	}
	return "", errors.New("Cgroup subsystem not found: " + cgroupToFind)
}

func getCurrentCGroupPath(cgroupsRoot, subsystem, tag string, privileged bool) string {
	parentCgroup := "garden"
	if tag != "" {
		parentCgroup = fmt.Sprintf("garden-%s", tag)
	}

	// We always use the cgroup root for privileged containers, regardless of
	// tag.
	if privileged {
		parentCgroup = ""
	}

	currentCgroup, err := getCurrentCGroup(subsystem)
	Expect(err).NotTo(HaveOccurred())
	return filepath.Join(cgroupsRoot, subsystem, currentCgroup, parentCgroup)
}

func containsElement(strings []string, elem string) bool {
	for _, e := range strings {
		if e == elem {
			return true
		}
	}
	return false
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

func createRootfsTar(modifyRootfs func(string)) string {
	return tarUpDir(createRootfs(modifyRootfs, 0755))
}

func createRootfs(modifyRootfs func(string), perm os.FileMode) string {
	var err error
	tmpDir, err := ioutil.TempDir("", "test-rootfs")
	Expect(err).NotTo(HaveOccurred())
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

func makeContainerd() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	containerdPath := filepath.Join(mustGetEnv("GOPATH"), filepath.FromSlash("src/github.com/containerd/containerd"))
	makeContainerdCommand := exec.Command("make")
	makeContainerdCommand.Env = append(os.Environ(), "BUILDTAGS=no_btrfs")
	runCommandInDir(makeContainerdCommand, containerdPath)
	return filepath.Join(containerdPath, "bin")
}

func jsonMarshal(v interface{}) []byte {
	buf := bytes.NewBuffer([]byte{})
	Expect(toml.NewEncoder(buf).Encode(v)).To(Succeed())
	return buf.Bytes()
}

func jsonUnmarshal(data []byte, v interface{}) {
	Expect(toml.Unmarshal(data, v)).To(Succeed())
}

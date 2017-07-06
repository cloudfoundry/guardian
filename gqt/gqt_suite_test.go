package gqt_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/pkg/locksmith"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"encoding/json"
	"testing"
)

var ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

var config runner.GdnRunnerConfig
var binaries runner.Binaries

// the unprivileged user is baked into the cfgarden/garden-ci-ubuntu image
var unprivilegedUID = uint32(5000)
var unprivilegedGID = uint32(5000)

var gqtStartTime time.Time

var defaultTestRootFS string

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		var err error
		binaries = runner.Binaries{}

		binaries.Tar = os.Getenv("GARDEN_TAR_PATH")

		binaries.Gdn, err = gexec.Build("code.cloudfoundry.org/guardian/cmd/gdn", "-tags", "daemon", "-race", "-ldflags", "-extldflags '-static'")
		Expect(err).NotTo(HaveOccurred())

		binaries.Init, err = gexec.Build("code.cloudfoundry.org/guardian/cmd/init")
		Expect(err).NotTo(HaveOccurred())

		binaries.NetworkPlugin, err = gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/fake_network_plugin")
		Expect(err).NotTo(HaveOccurred())

		binaries.ImagePlugin, err = gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/fake_image_plugin")
		Expect(err).NotTo(HaveOccurred())

		binaries.RuntimePlugin, err = gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/fake_runtime_plugin")
		Expect(err).NotTo(HaveOccurred())

		binaries.NoopPlugin, err = gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/noop_plugin")
		Expect(err).NotTo(HaveOccurred())

		if runtime.GOOS == "linux" {
			binaries.ExecRunner, err = gexec.Build("code.cloudfoundry.org/guardian/cmd/dadoo")
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command("make")
			cmd.Dir = "../rundmc/nstar"
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = GinkgoWriter
			Expect(cmd.Run()).To(Succeed())
			binaries.NSTar = "../rundmc/nstar/nstar"
		}

		data, err := json.Marshal(binaries)
		Expect(err).NotTo(HaveOccurred())

		return data
	}, func(data []byte) {
		Expect(json.Unmarshal(data, &binaries)).To(Succeed())
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
		Expect(os.Chmod(filepath.Join(binaries.Init, "..", ".."), 0755)).To(Succeed())
		filepath.Walk(filepath.Join(binaries.Init, "..", ".."), func(path string, info os.FileInfo, err error) error {
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(path, 0755)).To(Succeed())
			return nil
		})

		config = defaultConfig()
	})

	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

func defaultConfig() runner.GdnRunnerConfig {
	config := runner.DefaultGdnRunnerConfig()
	config.DefaultRootFS = defaultTestRootFS
	config.GdnBin = binaries.Gdn
	config.ExecRunnerBin = binaries.ExecRunner
	config.InitBin = binaries.Init
	config.TarBin = binaries.Tar
	config.NSTarBin = binaries.NSTar

	return config
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

package gqt_test

import (
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

var defaultRuntime = map[string]string{
	"linux":   "runc",
	"windows": "winc",
}

var ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

var binaries *runner.Binaries

// the unprivileged user is baked into the cfgarden/garden-ci-ubuntu image
var unprivilegedUID = uint32(5000)
var unprivilegedGID = uint32(5000)

var gqtStartTime time.Time

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		var err error
		binaries = &runner.Binaries{}

		binaries.OCIRuntime = os.Getenv("OCI_RUNTIME")
		if binaries.OCIRuntime == "" {
			binaries.OCIRuntime = defaultRuntime[runtime.GOOS]
		}

		binaries.Tar = os.Getenv("GARDEN_TAR_PATH")

		if binaries.OCIRuntime != "" {
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
		}

		data, err := json.Marshal(binaries)
		Expect(err).NotTo(HaveOccurred())

		return data
	}, func(data []byte) {
		binaries = &runner.Binaries{}
		Expect(json.Unmarshal(data, &binaries)).To(Succeed())
	})

	BeforeEach(func() {
		if binaries.OCIRuntime == "" {
			Skip("No OCI Runtime for Platform: " + runtime.GOOS)
		}

		if os.Getenv("GARDEN_TEST_ROOTFS") == "" {
			Skip("No Garden RootFS")
		}

		// chmod all the artifacts
		Expect(os.Chmod(filepath.Join(binaries.Init, "..", ".."), 0755)).To(Succeed())
		filepath.Walk(filepath.Join(binaries.Init, "..", ".."), func(path string, info os.FileInfo, err error) error {
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(path, 0755)).To(Succeed())
			return nil
		})
	})

	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

func startGarden(argv ...string) *runner.RunningGarden {
	return startGardenAsUser(nil, argv...)
}

func startGardenAsUser(user runner.UserCredential, argv ...string) *runner.RunningGarden {
	rootfs := os.Getenv("GARDEN_TEST_ROOTFS")
	return runner.Start(binaries, rootfs, user, argv...)
}

func restartGarden(client *runner.RunningGarden, argv ...string) *runner.RunningGarden {
	Expect(client.Ping()).To(Succeed(), "tried to restart garden while it was not running")
	Expect(client.Stop()).To(Succeed())
	return startGarden(argv...)
}

func startGardenWithoutDefaultRootfs(argv ...string) *runner.RunningGarden {
	return runner.Start(binaries, "", nil, argv...)
}

func runIPTables(ipTablesArgs ...string) ([]byte, error) {
	lock, err := locksmith.NewFileSystem().Lock(iptables.LockKey)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()
	return exec.Command("iptables", append([]string{"-w"}, ipTablesArgs...)...).CombinedOutput()
}

// returns the n'th ASCII character starting from 'a' through 'z'
// E.g. nodeToString(1) = a, nodeToString(2) = b, etc ...
func nodeToString(ginkgoNode int) string {
	r := 'a' + ginkgoNode - 1
	Expect(r).To(BeNumerically(">=", 'a'))
	Expect(r).To(BeNumerically("<=", 'z'))
	return string(r)
}

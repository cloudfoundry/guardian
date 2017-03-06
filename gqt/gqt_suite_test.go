package gqt_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/pkg/locksmith"
	"code.cloudfoundry.org/idmapper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"encoding/json"
	"testing"
)

var defaultRuntime = map[string]string{
	"linux": "runc",
}

var ginkgoIO = garden.ProcessIO{Stdout: GinkgoWriter, Stderr: GinkgoWriter}

var ociRuntimeBin, gardenBin, initBin, nstarBin, dadooBin, testImagePluginBin, testNetPluginBin, tarBin string

var unprivilegedUID uint32

var gqtStartTime time.Time

func TestGqt(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		gqtStartTime = time.Now()
		fmt.Printf("gqt began running at %s\n", gqtStartTime)

		var err error
		bins := make(map[string]string)

		bins["oci_runtime_path"] = os.Getenv("OCI_RUNTIME")
		if bins["oci_runtime_path"] == "" {
			bins["oci_runtime_path"] = defaultRuntime[runtime.GOOS]
		}

		if bins["oci_runtime_path"] != "" {
			gdnCompileStart := time.Now()
			fmt.Printf("began compiling gdn at %s\n", gdnCompileStart)
			bins["garden_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/gdn", "-tags", "daemon", "-race", "-ldflags", "-extldflags '-static'")
			gdnCompileTime := time.Since(gdnCompileStart)
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("gdn compile time: %s\n", gdnCompileTime)

			bins["dadoo_bin_bin_bin"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/dadoo")
			Expect(err).NotTo(HaveOccurred())

			bins["init_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/cmd/init")
			Expect(err).NotTo(HaveOccurred())

			bins["test_net_plugin_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/fake_network_plugin")
			Expect(err).NotTo(HaveOccurred())

			bins["test_image_plugin_bin_path"], err = gexec.Build("code.cloudfoundry.org/guardian/gqt/cmd/fake_image_plugin")
			Expect(err).NotTo(HaveOccurred())

			cmd := exec.Command("make")
			cmd.Dir = "../rundmc/nstar"
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = GinkgoWriter
			Expect(cmd.Run()).To(Succeed())
			bins["nstar_bin_path"] = "../rundmc/nstar/nstar"
		}

		data, err := json.Marshal(bins)
		Expect(err).NotTo(HaveOccurred())

		return data
	}, func(data []byte) {
		bins := make(map[string]string)
		Expect(json.Unmarshal(data, &bins)).To(Succeed())

		ociRuntimeBin = bins["oci_runtime_path"]
		gardenBin = bins["garden_bin_path"]
		nstarBin = bins["nstar_bin_path"]
		dadooBin = bins["dadoo_bin_bin_bin"]
		testImagePluginBin = bins["test_image_plugin_bin_path"]
		initBin = bins["init_bin_path"]
		testNetPluginBin = bins["test_net_plugin_bin_path"]

		tarBin = os.Getenv("GARDEN_TAR_PATH")
	})

	SynchronizedAfterSuite(func() {}, func() {
		fmt.Printf("gqt duration: %s\n", time.Since(gqtStartTime))
	})

	BeforeEach(func() {
		if ociRuntimeBin == "" {
			Skip("No OCI Runtime for Platform: " + runtime.GOOS)
		}

		if os.Getenv("GARDEN_TEST_ROOTFS") == "" {
			Skip("No Garden RootFS")
		}

		// chmod all the artifacts
		Expect(os.Chmod(filepath.Join(initBin, "..", ".."), 0755)).To(Succeed())
		filepath.Walk(filepath.Join(initBin, "..", ".."), func(path string, info os.FileInfo, err error) error {
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(path, 0755)).To(Succeed())
			return nil
		})

		// create /run/runc and chown to unprivileged user
		unprivilegedUID = uint32(idmapper.Min(idmapper.MustGetMaxValidUID(), idmapper.MustGetMaxValidGID()))
		runcRootDir := "/run/runc"
		Expect(os.MkdirAll(runcRootDir, 0700)).To(Succeed())
		Expect(os.Chown(runcRootDir, int(unprivilegedUID), int(unprivilegedUID))).To(Succeed())

	})

	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Suite")
}

func startGarden(argv ...string) *runner.RunningGarden {
	return startGardenAsUser(nil, argv...)
}

func startGardenAsUser(user *syscall.Credential, argv ...string) *runner.RunningGarden {
	rootfs := os.Getenv("GARDEN_TEST_ROOTFS")
	return runner.Start(gardenBin, initBin, nstarBin, dadooBin, testImagePluginBin, rootfs, tarBin, user, argv...)
}

func restartGarden(client *runner.RunningGarden, argv ...string) *runner.RunningGarden {
	Expect(client.Ping()).To(Succeed(), "tried to restart garden while it was not running")
	Expect(client.Stop()).To(Succeed())
	return startGarden(argv...)
}

func startGardenWithoutDefaultRootfs(argv ...string) *runner.RunningGarden {
	return runner.Start(gardenBin, initBin, nstarBin, dadooBin, testImagePluginBin, "", tarBin, nil, argv...)
}

func cleanupSystemResources(cgroupsMountpoint, iptablesPrefix string) error {
	umountCmd := exec.Command("sh", "-c", fmt.Sprintf("umount %s/*", cgroupsMountpoint))
	if err := umountCmd.Run(); err != nil {
		return err
	}
	umountCmd = exec.Command("sh", "-c", fmt.Sprintf("umount %s", cgroupsMountpoint))
	if err := umountCmd.Run(); err != nil {
		return err
	}

	cmd := exec.Command("bash", "-c", iptables.SetupScript)
	cmd.Env = []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
		"ACTION=teardown",
		"GARDEN_IPTABLES_BIN=/sbin/iptables",
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_INPUT_CHAIN=%s-input", iptablesPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_FORWARD_CHAIN=%s-forward", iptablesPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_DEFAULT_CHAIN=%s-default", iptablesPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_FILTER_INSTANCE_PREFIX=%s-instance-", iptablesPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_NAT_PREROUTING_CHAIN=%s-prerouting", iptablesPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_NAT_POSTROUTING_CHAIN=%s-postrounting", iptablesPrefix),
		fmt.Sprintf("GARDEN_IPTABLES_NAT_INSTANCE_PREFIX=%s-instance-", iptablesPrefix),
	}
	lock, err := locksmith.NewFileSystem().Lock(iptables.LockKey)
	if err != nil {
		return err
	}
	defer lock.Unlock()
	return cmd.Run()
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

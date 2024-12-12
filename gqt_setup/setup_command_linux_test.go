package gqt_setup_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/guardian/gqt/cgrouper"
	"code.cloudfoundry.org/guardian/gqt/runner"
	rundmccgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

var _ = Describe("gdn setup", func() {
	var (
		setupArgs    []string
		tag          string
		cgroupsRoot  string
		setupProcess *gexec.Session
	)

	BeforeEach(func() {
		// We want to test that "gdn setup" can mount the cgroup hierarchy.
		// "gdn server" without --skip-setup does this too, and most gqts implicitly
		// rely on it.
		// We need a new test "environment" regardless of what tests have previously
		// run with the same GinkgoParallelProcess.
		// There is also a 1 character limit on the tag due to iptables rule length
		// limitations.
		tag = nodeToString(GinkgoParallelProcess())
		setupArgs = []string{"setup", "--tag", tag}
		if cpuThrottlingEnabled() {
			setupArgs = append(setupArgs, "--enable-cpu-throttling")
		}
		cgroupsRoot = runner.CgroupsRootPath(tag)
		assertNotMounted(cgroupsRoot)
	})

	JustBeforeEach(func() {
		var err error

		cmd := exec.Command(binaries.Gdn, setupArgs...)
		setupProcess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(setupProcess, 10*time.Second).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		for {
			err := cgrouper.CleanGardenCgroups(cgroupsRoot, tag)
			if err == nil {
				break
			}
		}
	})

	Describe("cgroups", func() {
		It("sets up cgroups", func() {
			if cgroups.IsCgroup2UnifiedMode() {
				Skip("Skipping cgroups v1 tests when cgroups v2 is enabled")
			}
			mountpointCmd := exec.Command("mountpoint", "-q", cgroupsRoot+"/")
			mountpointCmd.Stdout = GinkgoWriter
			mountpointCmd.Stderr = GinkgoWriter

			Expect(mountpointCmd.Run()).To(Succeed(), fmt.Sprintf("cgroupsRoot: %q,\n mounts: %s", cgroupsRoot, getMountTable()))
		})

		It("allows both OCI default and garden specific devices", func() {
			if cgroups.IsCgroup2UnifiedMode() {
				Skip("Skipping cgroups v1 tests when cgroups v2 is enabled")
			}

			privileged := false
			cgroupPath, err := cgrouper.GetCGroupPath(cgroupsRoot, "devices", tag, privileged, cpuThrottlingEnabled())
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(time.Second * 20)

			content := readFile(filepath.Join(cgroupPath, "devices.list"))
			expectedAllowedDevices := []string{
				"c 1:3 rwm",
				"c 5:0 rwm",
				"c 1:8 rwm",
				"c 1:9 rwm",
				"c 1:5 rwm",
				"c 1:7 rwm",
				"c 10:229 rwm",
				"c *:* m",
				"b *:* m",
				"c 136:* rwm",
				"c 5:2 rwm",
			}
			contentLines := strings.Split(strings.TrimSpace(content), "\n")
			Expect(contentLines).To(HaveLen(len(expectedAllowedDevices)))
			Expect(contentLines).To(ConsistOf(expectedAllowedDevices))
		})

		Context("when CPU throttling is enabled", func() {
			BeforeEach(func() {
				if !cpuThrottlingEnabled() {
					Skip("only relevant when CPU throttling is enabled")
				}
			})

			It("creates the good cpu cgroup", func() {
				path, err := cgrouper.GetCGroupPath(cgroupsRoot, "cpu", tag, false, cpuThrottlingEnabled())
				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(BeADirectory())
				Expect(filepath.Base(path)).To(Equal(rundmccgroups.GoodCgroupName))
			})

			It("creates the bad cpu cgroup", func() {
				path, err := cgrouper.GetCGroupPath(cgroupsRoot, "cpu", tag, false, cpuThrottlingEnabled())
				Expect(err).NotTo(HaveOccurred())
				badCgroupPath := filepath.Join(path, "..", rundmccgroups.BadCgroupName)
				Expect(badCgroupPath).To(BeADirectory())
			})
		})
	})
})

func assertNotMounted(cgroupsRoot string) {
	mountsFileContent, err := os.ReadFile("/proc/self/mountinfo")
	Expect(err).NotTo(HaveOccurred())
	Expect(string(mountsFileContent)).NotTo(ContainSubstring(cgroupsRoot))
}

func getMountTable() string {
	output, err := exec.Command("cat", "/proc/self/mountinfo").Output()
	Expect(err).NotTo(HaveOccurred())

	return string(output)
}

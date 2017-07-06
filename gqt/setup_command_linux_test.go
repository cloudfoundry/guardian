package gqt_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("gdn setup", func() {
	var (
		tmpDir       string
		setupArgs    []string
		tag          string
		setupProcess *gexec.Session
	)

	BeforeEach(func() {
		tag = fmt.Sprintf("%d", GinkgoParallelNode())
		tmpDir = filepath.Join(
			os.TempDir(),
			fmt.Sprintf("test-garden-%d", GinkgoParallelNode()),
		)
		setupArgs = []string{"setup", "--tag", tag}
	})

	JustBeforeEach(func() {
		var err error

		cmd := exec.Command(binaries.Gdn, setupArgs...)
		cmd.Env = append(
			[]string{
				fmt.Sprintf("TMPDIR=%s", tmpDir),
				fmt.Sprintf("TEMP=%s", tmpDir),
				fmt.Sprintf("TMP=%s", tmpDir),
			},
			os.Environ()...,
		)
		setupProcess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess, 10*time.Second).Should(gexec.Exit(0))
	})

	Describe("mounting cgroups", func() {
		var cgroupsRoot string

		BeforeEach(func() {
			// We want to test that "gdn setup" can mount the cgroup hierarchy.
			// "gdn server" without --skip-setup does this too, and most gqts implicitly
			// rely on it.
			// We need a new test "environment" regardless of what tests have previously
			// run with the same GinkgoParallelNode.
			// There is also a 1 character limit on the tag due to iptables rule length
			// limitations.
			tag = nodeToString(GinkgoParallelNode())
			cgroupsRoot = filepath.Join(tmpDir, fmt.Sprintf("cgroups-%s", tag))
			ensureCgroupsForTagUnmounted(cgroupsRoot)
			tmpDir = filepath.Join(
				os.TempDir(),
				fmt.Sprintf("test-garden-%d", GinkgoParallelNode()),
			)
			setupArgs = []string{"setup", "--tag", tag}
		})

		It("sets up cgroups", func() {
			mountpointCmd := exec.Command("mountpoint", "-q", cgroupsRoot+"/")
			mountpointCmd.Stdout = GinkgoWriter
			mountpointCmd.Stderr = GinkgoWriter
			Expect(mountpointCmd.Run()).To(Succeed())
		})
	})

	Context("when we start the server", func() {
		var (
			server *runner.RunningGarden
		)

		BeforeEach(func() {
			config.SkipSetup = boolptr(true)
			config.Tag = tag
		})

		AfterEach(func() {
			Expect(server.DestroyAndStop()).To(Succeed())
		})

		Context("when the server is running as root", func() {
			JustBeforeEach(func() {
				config.User = &syscall.Credential{Uid: 0, Gid: 0}
				server = runner.Start(config)
				Expect(server).NotTo(BeNil())
			})

			It("should be able to create a container", func() {
				_, err := server.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when a dummy network plugin is suppplied", func() {
				BeforeEach(func() {
					config.NetworkPluginBin = "/bin/true"
				})

				It("should be able to create a container", func() {
					_, err := server.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})
})

func ensureCgroupsForTagUnmounted(cgroupsRoot string) {
	mountsFileContent, err := ioutil.ReadFile("/proc/self/mounts")
	Expect(err).NotTo(HaveOccurred())

	lines := strings.Split(string(mountsFileContent), "\n")

	tmpFsFound := false
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if fields[2] == "cgroup" && strings.Contains(fields[1], cgroupsRoot) {
			Expect(syscall.Unmount(fields[1], 0)).To(Succeed())
		}
		if fields[2] == "tmpfs" && fields[1] == cgroupsRoot {
			tmpFsFound = true
		}
	}

	if tmpFsFound {
		Expect(syscall.Unmount(cgroupsRoot, 0)).To(Succeed())
	}
}

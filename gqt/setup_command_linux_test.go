package gqt_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		tmpDir            string
		cgroupsMountpoint string
		setupArgs         []string
		tag               string
		setupProcess      *gexec.Session
	)

	BeforeEach(func() {
		// We want to test that "gdn setup" can mount the cgroup hierarchy.
		// "gdn server" without --skip-setup does this too, and most gqts implicitly
		// rely on it.
		// We need a new test "environment" regardless of what tests have previously
		// run with the same GinkgoParallelNode.
		// There is also a 1 character limit on the tag due to iptables rule length
		// limitations.
		tag = nodeToString(GinkgoParallelNode())
		tmpDir = filepath.Join(
			os.TempDir(),
			fmt.Sprintf("test-garden-%d", GinkgoParallelNode()),
		)
		cgroupsMountpoint = filepath.Join(tmpDir, fmt.Sprintf("cgroups-%s", tag))
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

	It("sets up cgroups", func() {
		mountpointCmd := exec.Command("mountpoint", "-q", cgroupsMountpoint+"/")
		mountpointCmd.Stdout = GinkgoWriter
		mountpointCmd.Stderr = GinkgoWriter
		Expect(mountpointCmd.Run()).To(Succeed())
	})

	Context("when we start the server", func() {
		var (
			server     *runner.RunningGarden
			serverArgs []string
		)

		BeforeEach(func() {
			serverArgs = []string{"--skip-setup", "--tag", fmt.Sprintf("%s", tag)}
		})

		AfterEach(func() {
			Expect(server.DestroyAndStop()).To(Succeed())
		})

		Context("when the server is running as root", func() {
			JustBeforeEach(func() {
				root := &syscall.Credential{Uid: 0, Gid: 0}
				server = startGardenAsUser(root, serverArgs...)
				Expect(server).NotTo(BeNil())
			})

			It("should be able to create a container", func() {
				_, err := server.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when a dummy network plugin is suppplied", func() {
				BeforeEach(func() {
					serverArgs = append(serverArgs, []string{"--network-plugin", "/bin/true"}...)
				})

				It("should be able to create a container", func() {
					_, err := server.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())
				})
			})
		})
	})
})

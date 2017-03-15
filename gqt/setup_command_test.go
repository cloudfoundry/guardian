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
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("gdn setup", func() {
	var (
		cgroupsMountpoint string
		iptablesPrefix    string
		setupArgs         []string
		tag               string
		setupProcess      *gexec.Session
	)

	BeforeEach(func() {
		// we can't use GinkgoParallelNode() directly here as this causes interference with the other tests in the GQT suite
		// i.e. beacuse for these specific tests, we want to teardown the iptables/cgroups after each test
		// and also because the --tag has a limitation of 1 char in length
		tag = nodeToString(GinkgoParallelNode())
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%s", tag))
		iptablesPrefix = fmt.Sprintf("w-%s", tag)
		setupArgs = []string{"setup", "--tag", tag}
	})

	JustBeforeEach(func() {
		var err error

		setupProcess, err = gexec.Start(exec.Command(gardenBin, setupArgs...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess, 10*time.Second).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(cleanupSystemResources(cgroupsMountpoint, iptablesPrefix)).To(Succeed())
	})

	It("sets up cgroups", func() {
		mountpointCmd := exec.Command("mountpoint", "-q", cgroupsMountpoint+"/")
		Expect(mountpointCmd.Run()).To(Succeed())
	})

	It("does not setup networking stuff", func() {
		out, err := runIPTables("-L", "INPUT")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring(iptablesPrefix + "-input"))

		out, err = runIPTables("-L", "FORWARD")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring(iptablesPrefix + "-forward"))
	})

	It("doesn't log spurious messages", func() {
		Consistently(setupProcess).ShouldNot(gbytes.Say("guardian-setup.iptables-runner.command.failed"))
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

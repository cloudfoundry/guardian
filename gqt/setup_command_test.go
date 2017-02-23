package gqt_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

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
		Eventually(setupProcess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(cleanupSystemResources(cgroupsMountpoint, iptablesPrefix)).To(Succeed())
	})

	It("sets up cgroups", func() {
		mountpointCmd := exec.Command("mountpoint", "-q", cgroupsMountpoint+"/")
		Expect(mountpointCmd.Run()).To(Succeed())
	})

	It("sets up iptables", func() {
		out, err := exec.Command("iptables", "-L", "INPUT").CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring(iptablesPrefix + "-input"))

		out, err = exec.Command("iptables", "-L", "FORWARD").CombinedOutput()
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring(iptablesPrefix + "-forward"))

		for _, suffix := range []string{"-input", "-default", "-forward"} {
			_, err := exec.Command("iptables", "-L", iptablesPrefix+suffix).CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("doesn't log spurious messages", func() {
		Consistently(setupProcess).ShouldNot(gbytes.Say("guardian-setup.iptables-runner.command.failed"))
	})

	Context("when --allow-host-access flag is passed", func() {
		BeforeEach(func() {
			setupArgs = append(setupArgs, []string{"--allow-host-access"}...)
		})

		It("iptables should have the relevant entry ", func() {
			out, err := exec.Command("iptables", "-L", iptablesPrefix+"-input").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(MatchRegexp("REJECT.*all.*reject-with icmp-host-prohibited"))
		})
	})

	Context("when --allow-host-access flag is passed", func() {
		BeforeEach(func() {
			setupArgs = append(setupArgs, "--deny-network", "8.8.8.0/24")
		})

		It("iptables should have the relevant entry ", func() {
			out, err := exec.Command("iptables", "-L", iptablesPrefix+"-default").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(MatchRegexp("REJECT.*8.8.8.0/24.*reject-with icmp-port-unreachable"))
		})
	})

	Context("when a binary is passed via --iptables-bin flag", func() {
		BeforeEach(func() {
			// use echo instead of iptables
			setupArgs = append(setupArgs, "--iptables-bin", "/bin/echo")
		})

		It("uses the binary passed instead of /sbin/iptables", func() {
			// chack all chains are empty
			out, err := exec.Command("iptables", "-L", "INPUT").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(MatchRegexp(iptablesPrefix + ".*anywhere"))

			out, err = exec.Command("iptables", "-L", "FORWARD").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(MatchRegexp(iptablesPrefix + ".*anywhere"))

			out, err = exec.Command("iptables", "-L", "OUTPUT").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(MatchRegexp(iptablesPrefix + ".*anywhere"))
		})
	})

	Context("when --reset-iptables-rules flag is passed", func() {
		var instanceChain string

		JustBeforeEach(func() {
			instanceChain = fmt.Sprintf("%s-instance-container", iptablesPrefix)
			Expect(exec.Command("iptables", "-N", instanceChain).Run()).To(Succeed())
		})

		It("iptables should have the relevant entry ", func() {
			Expect(exec.Command(gardenBin, append(setupArgs, "--reset-iptables-rules")...).Run()).To(Succeed())
			out, err := exec.Command("iptables", "-L").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			Expect(out).NotTo(ContainSubstring(instanceChain))
		})
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
		})
	})
})

package gqt_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/kawasaki/iptables"
	"code.cloudfoundry.org/guardian/sysinfo"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("gdn setup", func() {
	var (
		cgroupsMountpoint string
		iptablesPrefix    string
		setupArgs         []string
	)

	BeforeEach(func() {
		cgroupsMountpoint = filepath.Join(os.TempDir(), fmt.Sprintf("cgroups-%d", GinkgoParallelNode()))
		iptablesPrefix = fmt.Sprintf("w-%d", GinkgoParallelNode())
		setupArgs = []string{"setup", "--tag", fmt.Sprintf("%d", GinkgoParallelNode())}
	})

	JustBeforeEach(func() {
		setupProcess, err := gexec.Start(exec.Command(gardenBin, setupArgs...), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(setupProcess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		// cgroups
		umountCmd := exec.Command("sh", "-c", fmt.Sprintf("umount %s/*", cgroupsMountpoint))
		Expect(umountCmd.Run()).To(Succeed(), "unmount %s", cgroupsMountpoint)
		umountCmd = exec.Command("sh", "-c", fmt.Sprintf("umount %s", cgroupsMountpoint))
		Expect(umountCmd.Run()).To(Succeed(), "unmount %s", cgroupsMountpoint)

		// clean up iptables rules
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
		Expect(cmd.Run()).To(Succeed())
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

	Context("when start the server", func() {
		var (
			server     *runner.RunningGarden
			serverArgs []string
		)

		BeforeEach(func() {
			serverArgs = []string{"--skip-setup", "--tag", fmt.Sprintf("%d", GinkgoParallelNode())}
		})

		Context("when server is as non-root", func() {
			var maximus *syscall.Credential

			BeforeEach(func() {
				maxId := uint32(sysinfo.Min(sysinfo.MustGetMaxValidUID(), sysinfo.MustGetMaxValidGID()))
				maximus = &syscall.Credential{Uid: maxId, Gid: maxId}
			})

			JustBeforeEach(func() {
				serverArgs = append(serverArgs, "--image-plugin", testImagePluginBin)
				server = startGardenAsUser(maximus, serverArgs...)
				Expect(server).NotTo(BeNil())
			})

			It("server process should run consistently as non-root user", func() {
				out, err := exec.Command("ps", "-U", fmt.Sprintf("%d", maximus.Uid)).CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), "No process of user maximus was found")
				Expect(out).To(ContainSubstring(fmt.Sprintf("%d", server.Pid)))

				Consistently(func() error {
					return exec.Command("ps", "-p", strconv.Itoa(server.Pid)).Run()
				}).Should(Succeed())

			})
		})

		Context("when server is running as root", func() {
			JustBeforeEach(func() {
				root := &syscall.Credential{Uid: 0, Gid: 0}
				server = startGardenAsUser(root, serverArgs...)
				Expect(server).NotTo(BeNil())
			})

			AfterEach(func() {
				server.DestroyAndStop()
			})

			It("should be able to create a container", func() {
				_, err := server.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

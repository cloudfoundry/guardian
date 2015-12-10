package iptables_test

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PortForwarder", func() {
	var (
		spec            *kawasaki.PortForwarderSpec
		netCfg          *kawasaki.NetworkConfig
		forwarder       *iptables.PortForwarder
		containerHandle string
		externalPort    uint32
		containerPort   uint32
	)

	BeforeEach(func() {
		externalPort = uint32(1210 + GinkgoParallelNode())
		containerPort = uint32(2120 + GinkgoParallelNode())
		containerHandle = fmt.Sprintf("h-%d", GinkgoParallelNode())
		netCfg = &kawasaki.NetworkConfig{}
	})

	JustBeforeEach(func() {
		spec = &kawasaki.PortForwarderSpec{
			NetworkConfig: netCfg,
			FromPort:      externalPort,
			ToPort:        containerPort,
		}
		forwarder = iptables.NewPortForwarder(linux_command_runner.New())
	})

	Context("when NetworkConfig is valid", func() {
		BeforeEach(func() {
			bridgeIp, subnet, err := net.ParseCIDR("127.0.0.1/32")
			Expect(err).NotTo(HaveOccurred())

			containerIP := net.ParseIP("127.0.0.2")
			Expect(containerIP).NotTo(BeNil())

			netCfg = &kawasaki.NetworkConfig{
				IPTableChain: fmt.Sprintf("chain-%s", containerHandle),
				BridgeIP:     bridgeIp,
				ContainerIP:  containerIP,
				Subnet:       subnet,
			}

			createChainCmd := exec.Command("iptables", "-w", "-t", "nat", "-N", netCfg.IPTableChain)
			Expect(createChainCmd.Run()).To(Succeed())
		})

		AfterEach(func() {
			// clean up rules created by PortForwarder
			_, maskSize := netCfg.Subnet.Mask.Size()
			deleteRuleCmd := exec.Command(
				"iptables", "-w", "-t", "nat",
				"-D", netCfg.IPTableChain,
				"-d", fmt.Sprintf("%s/%d", netCfg.BridgeIP.String(), maskSize),
				"-p", "tcp",
				"-m", "tcp",
				"--dport", fmt.Sprint(externalPort),
				"-j", "DNAT",
				"--to-destination", fmt.Sprintf("%s:%d", netCfg.ContainerIP.String(), containerPort),
			)
			Expect(deleteRuleCmd.Run()).To(Succeed())

			deleteChainCmd := exec.Command(
				"iptables", "-w", "-t", "nat",
				"-X", netCfg.IPTableChain,
			)
			Expect(deleteChainCmd.Run()).To(Succeed())
		})

		It("creates an iptables chain and adds a rule to it", func() {
			err := forwarder.Forward(spec)
			Expect(err).NotTo(HaveOccurred())

			out, err := exec.Command("iptables", "--table", "nat", "-S").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())

			ipTableRules := string(out)

			Expect(ipTableRules).To(ContainSubstring(netCfg.IPTableChain))
			Expect(ipTableRules).To(ContainSubstring(containerHandle))
			Expect(ipTableRules).To(ContainSubstring(fmt.Sprint(externalPort)))
			Expect(ipTableRules).To(ContainSubstring(fmt.Sprint(containerPort)))
			Expect(ipTableRules).To(ContainSubstring(netCfg.BridgeIP.String()))
			Expect(ipTableRules).To(ContainSubstring(netCfg.ContainerIP.String()))
		})
	})

	Context("when NetworkConfig is invalid", func() {
		BeforeEach(func() {
			netCfg = &kawasaki.NetworkConfig{IPTableChain: fmt.Sprintf("chain-%s", containerHandle)}
		})

		It("returns the error", func() {
			err := forwarder.Forward(spec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Bad IP address"))
		})
	})

})

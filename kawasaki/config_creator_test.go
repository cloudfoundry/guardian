package kawasaki_test

import (
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ConfigCreator", func() {
	var creator *kawasaki.Creator
	var subnet *net.IPNet
	var ip net.IP
	var logger lager.Logger

	BeforeEach(func() {
		var err error
		ip, subnet, err = net.ParseCIDR("192.168.12.20/24")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test")

		creator = kawasaki.NewConfigCreator("intf-prefix", "chain-prefix")
	})

	It("assigns the bridge name based on the subnet", func() {
		config, err := creator.Create(logger, "banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.BridgeName).To(Equal("br-192-168-12-0"))
	})

	Context("when the handle is short", func() {
		It("assigns the correct interface names", func() {
			config, err := creator.Create(logger, "banana", subnet, ip)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.HostIntf).To(Equal("intf-prefix-banana-0"))
			Expect(config.ContainerIntf).To(Equal("intf-prefix-banana-1"))
		})

		It("assigns the correct instance chain name", func() {
			config, err := creator.Create(logger, "banana", subnet, ip)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.IPTableChain).To(Equal("chain-prefix-banana"))
		})
	})

	Context("when the handle is long", func() {
		It("truncates the interface names", func() {
			config, err := creator.Create(logger, "bananashmanana", subnet, ip)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.HostIntf).To(Equal("intf-prefix-bananash-0"))
			Expect(config.ContainerIntf).To(Equal("intf-prefix-bananash-1"))
		})

		It("truncates the name to create the iptable chain name", func() {
			config, err := creator.Create(logger, "bananashmanana", subnet, ip)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.IPTableChain).To(Equal("chain-prefix-bananash"))
		})
	})

	It("saves the subnet and ip", func() {
		config, err := creator.Create(logger, "banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.ContainerIP.String()).To(Equal("192.168.12.20"))
		Expect(config.Subnet.String()).To(Equal("192.168.12.0/24"))
	})

	It("assigns the bridge IP as the first IP in the subnet", func() {
		config, err := creator.Create(logger, "banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.BridgeIP.String()).To(Equal("192.168.12.1"))
	})

	It("hard-codes the MTU to 1500", func() {
		config, err := creator.Create(logger, "banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.Mtu).To(Equal(1500))
	})
})

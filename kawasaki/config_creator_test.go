package kawasaki_test

import (
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigCreator", func() {
	var creator *kawasaki.Creator
	var subnet *net.IPNet
	var ip net.IP

	BeforeEach(func() {
		var err error
		ip, subnet, err = net.ParseCIDR("192.168.12.20/24")
		Expect(err).NotTo(HaveOccurred())

		creator = kawasaki.NewConfigCreator()
	})

	It("assigns the bridge name based on the subnet", func() {
		config, err := creator.Create("banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.BridgeName).To(Equal("br-192-168-12-0"))
	})

	Context("when the interface names are short", func() {
		It("assigns the correct interface names", func() {
			config, err := creator.Create("banana", subnet, ip)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.HostIntf).To(Equal("w-banana-0"))
			Expect(config.ContainerIntf).To(Equal("w-banana-1"))
		})
	})

	Context("when the interface names are long", func() {
		It("truncates the names", func() {
			config, err := creator.Create("bananashmanana", subnet, ip)
			Expect(err).NotTo(HaveOccurred())

			Expect(config.HostIntf).To(Equal("w-bananash-0"))
			Expect(config.ContainerIntf).To(Equal("w-bananash-1"))
		})
	})

	It("saves the subnet and ip", func() {
		config, err := creator.Create("banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.ContainerIP.String()).To(Equal("192.168.12.20"))
		Expect(config.Subnet.String()).To(Equal("192.168.12.0/24"))
	})

	It("assigns the bridge IP as the first IP in the subnet", func() {
		config, err := creator.Create("banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.BridgeIP.String()).To(Equal("192.168.12.1"))
	})

	It("hard-codes the MTU to 1500", func() {
		config, err := creator.Create("banana", subnet, ip)
		Expect(err).NotTo(HaveOccurred())

		Expect(config.Mtu).To(Equal(1500))
	})
})

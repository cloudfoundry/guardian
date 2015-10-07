package kawasaki_test

import (
	"io/ioutil"
	"net"
	"os"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// type NetworkConfig struct {
// 	HostIntf      string
// 	ContainerIntf string
// 	BridgeName    string
// 	BridgeIP      net.IP
// 	ContainerIP   net.IP
// 	Subnet        *net.IPNet
// 	Netns         *os.File
// 	NetnsPath     string
// 	Mtu           int
// }

var _ = Describe("ConfigCreator", func() {
	var creator *kawasaki.Creator
	var netnsFD *os.File

	BeforeEach(func() {
		_, networkPool, err := net.ParseCIDR("10.1.1.0/30")
		Expect(err).NotTo(HaveOccurred())
		creator = kawasaki.NewConfigCreator(networkPool)

		netnsFD, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.Remove(netnsFD.Name())
	})

	Context("when the interface names are short", func() {
		It("assigns the correct interface names", func() {
			config, err := creator.Create("banana", "192.168.12.20/24")
			Expect(err).NotTo(HaveOccurred())

			Expect(config.HostIntf).To(Equal("w-banana-0"))
			Expect(config.ContainerIntf).To(Equal("w-banana-1"))
			Expect(config.BridgeName).To(Equal("br-banana"))
		})
	})

	Context("when the interface names are long", func() {
		It("truncates the names", func() {
			config, err := creator.Create("bananashmanana", "192.168.12.20/24")
			Expect(err).NotTo(HaveOccurred())

			Expect(config.HostIntf).To(Equal("w-bananash-0"))
			Expect(config.ContainerIntf).To(Equal("w-bananash-1"))
		})
	})

	Context("when the spec is empty", func() {
		It("assigns the IP and subnet based on the pass-in network pool", func() {
			config, err := creator.Create("banana", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(config.BridgeIP.String()).To(Equal("10.1.1.1"))
			Expect(config.ContainerIP.String()).To(Equal("10.1.1.2"))
			Expect(config.Subnet.String()).To(Equal("10.1.1.0/30"))
		})
	})

	Context("when the spec contains a CIDR", func() {
		It("assigns the IP and subnet based on the spec", func() {
			config, err := creator.Create("banana", "192.168.12.20/24")
			Expect(err).NotTo(HaveOccurred())

			Expect(config.ContainerIP.String()).To(Equal("192.168.12.20"))
			Expect(config.Subnet.String()).To(Equal("192.168.12.0/24"))
		})

		It("assigns the bridge IP as the first IP in the subnet", func() {
			config, err := creator.Create("banana", "192.168.12.20/24")
			Expect(err).NotTo(HaveOccurred())

			Expect(config.BridgeIP.String()).To(Equal("192.168.12.1"))
		})
	})

	It("hard-codes the MTU to 1500", func() {
		config, err := creator.Create("banana", "192.168.12.20/24")
		Expect(err).NotTo(HaveOccurred())

		Expect(config.Mtu).To(Equal(1500))
	})

	It("returns errors if the spec cannot be parsed as a CIDR", func() {
		_, err := creator.Create("foo", "!!")
		Expect(err).To(MatchError("invalid CIDR address: !!"))
	})

	PContext("when the container handle is long", func() {})
	PContext("when the spec IP is the fist IP of the subnet", func() {})
})

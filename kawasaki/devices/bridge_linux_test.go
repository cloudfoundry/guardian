package devices_test

import (
	"fmt"
	"io/ioutil"
	"net"

	"code.cloudfoundry.org/guardian/kawasaki/devices"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bridge Management", func() {
	var (
		b      devices.Bridge
		name   string
		addr   string
		ip     net.IP
		subnet *net.IPNet
	)

	BeforeEach(func() {
		name = fmt.Sprintf("gdn-test-intf-%d", GinkgoParallelNode())

		var err error
		addr = "10.9.0.1/30"
		ip, subnet, err = net.ParseCIDR(addr)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		cleanup(name)
	})

	Describe("Create", func() {
		Context("when the bridge does not already exist", func() {
			It("creates a bridge", func() {
				_, err := b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())
			})

			It("sets the bridge name", func() {
				bridge, err := b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())

				Expect(bridge.Name).To(Equal(name))
			})

			It("assigns a mac address so that the bridge address does not change", func() {
				_, err := b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())

				// 1 means the kernel auto-assigns a mac address based on the lowest-attached
				// device. This is bad because it can change (!) when devices are added/removed
				// and mess stuff up. 3 means we assigned an explicit address so this shouldnt happen.
				Expect(ioutil.ReadFile(fmt.Sprintf("/sys/class/net/%s/addr_assign_type", name))).To(Equal([]byte("3\n")))
			})

			It("sets the bridge address", func() {
				bridge, err := b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())

				addrs, err := bridge.Addrs()
				Expect(err).ToNot(HaveOccurred())

				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].String()).To(Equal(addr))
			})
		})

		Context("when the bridge exists", func() {
			var (
				existingIfc *net.Interface
			)
			BeforeEach(func() {
				var err error
				existingIfc, err = b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when the bridge is duplicated", func() {
				It("returns the interface", func() {
					infc, err := b.Create(name, ip, subnet)
					Expect(err).ToNot(HaveOccurred())
					Expect(infc).To(Equal(existingIfc))
				})
			})

			It("returns the interface for it", func() {
				ifc, err := b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())
				Expect(ifc).To(Equal(existingIfc))
			})

			It("does not change the existing bridge", func() {
				ip2, subnet2, _ := net.ParseCIDR("10.8.0.2/30")
				_, err := b.Create(name, ip2, subnet2)
				Expect(err).ToNot(HaveOccurred())

				intf, err := net.InterfaceByName(name)
				Expect(err).ToNot(HaveOccurred())

				addrs, err := intf.Addrs()
				Expect(err).ToNot(HaveOccurred())

				Expect(addrs[0].String()).To(Equal(addr))
			})
		})
	})

	Describe("Add", func() {
		Context("when the slave does not exist", func() {
			It("returns the error", func() {
				existingIfc, err := b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())

				slave := &net.Interface{Name: "does not exist"}
				Expect(b.Add(existingIfc, slave)).To(MatchError("Link not found"))
			})
		})

		Context("when the bridge does not exist", func() {
			It("returns the error", func() {
				interfaces, err := net.Interfaces()
				Expect(err).NotTo(HaveOccurred())
				Expect(interfaces).NotTo(HaveLen(0))

				slave := &interfaces[0]
				bridge := &net.Interface{Name: "does not exist"}
				Expect(b.Add(bridge, slave)).To(MatchError("Link not found"))
			})
		})
	})

	Describe("Destroy", func() {
		Context("when the bridge exists", func() {
			It("deletes it", func() {
				br, err := b.Create(name, ip, subnet)
				Expect(err).ToNot(HaveOccurred())

				// sanity check
				Expect(interfaceNames()).To(ContainElement(name))

				// delete
				Expect(b.Destroy(br.Name)).To(Succeed())

				// should be gone
				Eventually(interfaceNames).ShouldNot(ContainElement(name))
			})
		})

		Context("when the bridge does not exist", func() {
			It("does not return an error (because Destroy should be idempotent)", func() {
				Expect(b.Destroy("something")).To(Succeed())
			})
		})
	})
})

func interfaceNames() []string {
	intfs, err := net.Interfaces()
	Expect(err).ToNot(HaveOccurred())

	v := make([]string, 0)
	for _, i := range intfs {
		v = append(v, i.Name)
	}

	return v
}

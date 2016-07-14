package devices_test

import (
	"fmt"
	"net"

	"code.cloudfoundry.org/guardian/kawasaki/devices"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Veth Pair Creation", func() {
	var (
		v                       devices.VethCreator
		hostName, containerName string
	)

	f := func(i *net.Interface, _ error) *net.Interface {
		return i
	}

	l := func(_, _ interface{}, e error) error {
		return e
	}

	BeforeEach(func() {
		hostName = fmt.Sprintf("doesntexist-h-%d", GinkgoParallelNode())
		containerName = fmt.Sprintf("doesntexist-c-%d", GinkgoParallelNode())
	})

	AfterEach(func() {
		Expect(cleanup(hostName)).To(Succeed())
		Expect(cleanup(containerName)).To(Succeed())
	})

	Context("when neither host already exists", func() {
		It("creates both interfaces in the host", func() {
			Expect(l(v.Create(hostName, containerName))).To(Succeed())
			Expect(net.InterfaceByName(hostName)).ToNot(BeNil())
			Expect(net.InterfaceByName(containerName)).ToNot(BeNil())
		})

		It("returns the created interfaces", func() {
			a, b, err := v.Create(hostName, containerName)
			Expect(err).ToNot(HaveOccurred())

			Expect(a).To(Equal(f(net.InterfaceByName(hostName))))
			Expect(b).To(Equal(f(net.InterfaceByName(containerName))))
		})
	})

	Context("when one of the interfaces already exists", func() {
		It("returns an error", func() {
			Expect(l(v.Create(hostName, containerName))).To(Succeed())
			Expect(l(v.Create(hostName, containerName))).ToNot(Succeed())
		})
	})
})

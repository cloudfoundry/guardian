package kawasaki_test

import (
	"net"

	"code.cloudfoundry.org/guardian/kawasaki"
	"code.cloudfoundry.org/guardian/kawasaki/subnets"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseSpec", func() {
	Context("when the spec is empty", func() {
		It("returns a dynamic subnet and ip", func() {
			subnetReq, ipReq, err := kawasaki.ParseSpec("")
			Expect(err).ToNot(HaveOccurred())

			Expect(subnetReq).To(Equal(subnets.DynamicSubnetSelector))
			Expect(ipReq).To(Equal(subnets.DynamicIPSelector))
		})
	})

	Context("when the network parameter is not empty", func() {
		Context("when it contains a prefix length", func() {
			It("statically allocates the requested subnet ", func() {
				subnetReq, ipReq, err := kawasaki.ParseSpec("1.2.3.0/30")
				Expect(err).ToNot(HaveOccurred())

				_, sn, _ := net.ParseCIDR("1.2.3.0/30")
				Expect(subnetReq).To(Equal(subnets.StaticSubnetSelector{IPNet: sn}))
				Expect(ipReq).To(Equal(subnets.DynamicIPSelector))
			})
		})

		Context("when it does not contain a prefix length", func() {
			It("statically allocates the requested Network from Subnets as a /30", func() {
				subnetReq, ipReq, err := kawasaki.ParseSpec("1.2.3.0")
				Expect(err).ToNot(HaveOccurred())

				_, sn, _ := net.ParseCIDR("1.2.3.0/30")
				Expect(subnetReq).To(Equal(subnets.StaticSubnetSelector{IPNet: sn}))
				Expect(ipReq).To(Equal(subnets.DynamicIPSelector))
			})
		})

		Context("when the network parameter has non-zero host bits", func() {
			It("statically allocates an IP address based on the network parameter", func() {
				subnetReq, ipReq, err := kawasaki.ParseSpec("1.2.3.1/20")
				Expect(err).ToNot(HaveOccurred())

				_, sn, _ := net.ParseCIDR("1.2.3.0/20")
				Expect(subnetReq).To(Equal(subnets.StaticSubnetSelector{IPNet: sn}))
				Expect(ipReq).To(Equal(subnets.StaticIPSelector{IP: net.ParseIP("1.2.3.1")}))
			})
		})

		Context("when the network parameter has zero host bits", func() {
			It("dynamically allocates an IP address", func() {
				subnetReq, ipReq, err := kawasaki.ParseSpec("1.2.3.0/24")
				Expect(err).ToNot(HaveOccurred())

				_, sn, _ := net.ParseCIDR("1.2.3.0/24")
				Expect(subnetReq).To(Equal(subnets.StaticSubnetSelector{IPNet: sn}))
				Expect(ipReq).To(Equal(subnets.DynamicIPSelector))
			})
		})

		Context("when an invalid network string is passed", func() {
			It("returns an error", func() {
				_, _, err := kawasaki.ParseSpec("not a network")
				Expect(err).To(MatchError("invalid CIDR address: not a network/30"))
			})
		})
	})
})

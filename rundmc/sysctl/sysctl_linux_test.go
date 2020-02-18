package sysctl_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/rundmc/sysctl"
)

var _ = Describe("Sysctl", func() {
	var s *sysctl.Sysctl

	BeforeEach(func() {
		s = sysctl.New()
	})

	Describe("Get", func() {
		It("gets the value of a sysctl kernel parameter", func() {
			value, err := s.Get("net.ipv4.tcp_keepalive_time")

			Expect(err).NotTo(HaveOccurred())
			Expect(value).To(BeNumerically(">", 0))
		})

		When("it fails to retrieve the property", func() {
			It("fails", func() {
				_, err := s.Get("foo.bar")

				Expect(err).To(HaveOccurred())
			})
		})

		When("the property is not an integer", func() {
			It("fails", func() {
				_, err := s.Get("kernel.ostype")

				Expect(err).To(HaveOccurred())
			})
		})
	})
})

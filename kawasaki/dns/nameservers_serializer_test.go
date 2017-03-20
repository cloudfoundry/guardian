package dns_test

import (
	. "code.cloudfoundry.org/guardian/kawasaki/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NameserversSerializer", func() {
	It("returns the list of nameservers in resolv file format", func() {
		serializer := NameserversSerializer{}
		resolvContents := serializer.Serialize(ips("1.2.3.4", "5.6.7.8"))
		Expect(string(resolvContents)).To(Equal("nameserver 1.2.3.4\nnameserver 5.6.7.8\n"))
	})
})

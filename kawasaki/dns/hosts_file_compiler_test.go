package dns_test

import (
	"net"

	. "github.com/cloudfoundry-incubator/guardian/kawasaki/dns"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HostsFileCompiler", func() {
	var (
		compiler HostsFileCompiler

		log lager.Logger
	)

	BeforeEach(func() {
		compiler = HostsFileCompiler{
			Handle: "my-handle",
			IP:     net.ParseIP("123.124.126.128"),
		}

		log = lagertest.NewTestLogger("test")
	})

	Describe("Compile", func() {
		It("should configure the localhost mapping", func() {
			contents, err := compiler.Compile(log)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("127.0.0.1 localhost"))
		})

		It("should configure the hostname mapping", func() {
			contents, err := compiler.Compile(log)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("123.124.126.128 my-handle"))
		})
	})
})

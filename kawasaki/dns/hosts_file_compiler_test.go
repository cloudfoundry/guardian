package dns_test

import (
	"net"

	. "code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HostsFileCompiler", func() {
	var (
		compiler HostsFileCompiler
		ip       net.IP

		log lager.Logger
	)

	BeforeEach(func() {
		ip = net.ParseIP("123.124.126.128")
		compiler = HostsFileCompiler{}
		log = lagertest.NewTestLogger("test")
	})

	Describe("Compile", func() {
		It("should configure the localhost mapping", func() {
			contents, err := compiler.Compile(log, ip, "myhandle")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("127.0.0.1 localhost"))
		})

		It("should configure the hostname mapping", func() {
			contents, err := compiler.Compile(log, ip, "my-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("123.124.126.128 my-handle"))
		})

		Context("when handle is longer than 49 characters", func() {
			It("should use the last 49 characters of it", func() {
				contents, err := compiler.Compile(log, ip, "too-looooong-haaaaaaaaaaaaaannnnnndddle-1234456787889")
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(ContainSubstring("123.124.126.128 looooong-haaaaaaaaaaaaaannnnnndddle-1234456787889"))
			})
		})
	})
})

package dns_test

import (
	"net"
	"strings"

	. "code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
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
		It("should configure the ipv4 localhost mapping", func() {
			contents, err := compiler.Compile(log, ip, "myhandle", []string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("127.0.0.1 localhost"))
		})

		It("should configure the hostname mapping", func() {
			contents, err := compiler.Compile(log, ip, "my-handle", []string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("123.124.126.128 my-handle"))
		})

		It("should configure the additional hosts", func() {
			additionalHosts := []string{
				"1.2.3.4 foo",
				"2.3.4.5 bar"}

			contents, err := compiler.Compile(log, ip, "myhandle", additionalHosts)
			Expect(err).NotTo(HaveOccurred())
			hosts := strings.Split(string(contents), "\n")
			hosts = hosts[:len(hosts)-1]

			Expect(hosts[len(hosts)-2]).To(Equal(additionalHosts[0]))
			Expect(hosts[len(hosts)-1]).To(Equal(additionalHosts[1]))
		})

		Context("when handle is longer than 49 characters", func() {
			It("should use the last 49 characters of it", func() {
				contents, err := compiler.Compile(log, ip, "too-looooong-haaaaaaaaaaaaaannnnnndddle-1234456787889", []string{})
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(ContainSubstring("123.124.126.128 looooong-haaaaaaaaaaaaaannnnnndddle-1234456787889"))
			})
		})
	})
})

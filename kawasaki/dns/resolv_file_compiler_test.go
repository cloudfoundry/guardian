package dns_test

import (
	"io/ioutil"
	"net"
	"os"

	"code.cloudfoundry.org/guardian/kawasaki/dns"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolvFileCompiler", func() {
	var (
		hostResolvConfPath string

		log      lager.Logger
		hostIp   net.IP
		compiler *dns.ResolvFileCompiler
	)

	BeforeEach(func() {
		log = lagertest.NewTestLogger("test")
		hostIp = net.ParseIP("254.253.252.251")

		compiler = &dns.ResolvFileCompiler{}
	})

	writeFile := func(filePath, contents string) {
		f, err := os.Create(filePath)
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()

		_, err = f.Write([]byte(contents))
		Expect(err).NotTo(HaveOccurred())
	}

	Context("when the host resolv.conf file does not exist", func() {
		BeforeEach(func() {
			hostResolvConfPath = "/does/not/exist.conf"
		})

		It("should return an error", func() {
			_, err := compiler.Compile(log, hostResolvConfPath, hostIp, nil, nil)
			Expect(err).To(MatchError(ContainSubstring(("reading file '/does/not/exist.conf'"))))
		})
	})

	Context("when the host resolv.conf exists", func() {
		var (
			overrideServers      []net.IP
			additionalDNSServers []net.IP
			contents             []byte
		)

		BeforeEach(func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
			hostResolvConfPath = f.Name()

			overrideServers = []net.IP{}
			additionalDNSServers = []net.IP{}
		})

		JustBeforeEach(func() {
			var err error
			contents, err = compiler.Compile(log, hostResolvConfPath, hostIp, overrideServers, additionalDNSServers)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.Remove(hostResolvConfPath)).To(Succeed())
		})

		Context("and explicit overrides are given", func() {
			BeforeEach(func() {
				overrideServers = []net.IP{
					net.ParseIP("8.8.8.8"),
					net.ParseIP("127.0.0.4"),
				}
			})

			It("writes the DNS entries to the container's resolv.conf", func() {
				Expect(string(contents)).To(Equal("nameserver 8.8.8.8\nnameserver 127.0.0.4\n"))
			})

			Context("and additional dns servers are provided", func() {
				BeforeEach(func() {
					additionalDNSServers = []net.IP{
						net.ParseIP("1.2.3.4"),
						net.ParseIP("9.8.7.6"),
					}
				})

				It("appends the additional dns entries to the container's resolv.conf", func() {
					Expect(string(contents)).To(Equal("nameserver 8.8.8.8\nnameserver 127.0.0.4\nnameserver 1.2.3.4\nnameserver 9.8.7.6\n"))
				})
			})
		})

		Context("and the host has only 1 resolv entry and it's local", func() {
			BeforeEach(func() {
				writeFile(hostResolvConfPath, "nameserver 127.0.0.1\n")
			})

			It("writes the host IP to the container's resolv.conf", func() {
				Expect(string(contents)).To(Equal("nameserver 254.253.252.251\n"))
			})

			Context("and additional dns servers are provided", func() {
				BeforeEach(func() {
					additionalDNSServers = []net.IP{
						net.ParseIP("1.2.3.4"),
						net.ParseIP("9.8.7.6"),
					}
				})

				It("appends the additional dns entries to the host IP", func() {
					Expect(string(contents)).To(Equal("nameserver 254.253.252.251\nnameserver 1.2.3.4\nnameserver 9.8.7.6\n"))
				})
			})
		})

		Context("and the host has only 1 resolv entry and it's not local", func() {
			BeforeEach(func() {
				writeFile(hostResolvConfPath, "nameserver 8.8.8.8\n")
			})

			It("copies the host's resolv.conf", func() {
				Expect(string(contents)).To(Equal("nameserver 8.8.8.8\n"))
			})
		})

		Context("and the host has many resolv entries including a local", func() {
			var expectedResolvConfContents string

			BeforeEach(func() {
				resolvConfContents := "nameserver 127.0.0.1\nnameserver 8.8.4.4\nnameserver 8.8.8.8\n"
				expectedResolvConfContents = "nameserver 8.8.4.4\nnameserver 8.8.8.8\n"
				writeFile(hostResolvConfPath, resolvConfContents)
			})

			It("copies the host's resolv.conf except for local entries", func() {
				Expect(string(contents)).To(Equal(expectedResolvConfContents))
			})

			Context("and additional dns servers are provided", func() {
				BeforeEach(func() {
					additionalDNSServers = []net.IP{
						net.ParseIP("1.2.3.4"),
						net.ParseIP("9.8.7.6"),
					}
				})

				It("appends the additional dns entries to the container's resolv.conf", func() {
					Expect(string(contents)).To(Equal("nameserver 8.8.4.4\nnameserver 8.8.8.8\nnameserver 1.2.3.4\nnameserver 9.8.7.6\n"))
				})
			})
		})
	})
})

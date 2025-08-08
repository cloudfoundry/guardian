package kawasaki_test

import (
	"errors"
	"net"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/kawasaki"
	fakes "code.cloudfoundry.org/guardian/kawasaki/kawasakifakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolvConfigurer", func() {
	var (
		log                   lager.Logger
		fakeHostsFileCompiler *fakes.FakeHostFileCompiler
		fakeResolvCompiler    *fakes.FakeResolvCompiler
		tmpDir                string
		depotDir              string
		handle                = "some-container"

		dnsResolv *kawasaki.ResolvConfigurer
	)

	BeforeEach(func() {
		log = lagertest.NewTestLogger("test")
		fakeHostsFileCompiler = new(fakes.FakeHostFileCompiler)
		fakeResolvCompiler = new(fakes.FakeResolvCompiler)

		var err error
		tmpDir, err = os.MkdirTemp("", "resolv-test")
		Expect(err).NotTo(HaveOccurred())
		resolvFilePath := filepath.Join(tmpDir, "host-resolv.conf")
		Expect(os.WriteFile(resolvFilePath, []byte("nameserver 1.2.3.4\n"), 0755)).To(Succeed())
		depotDir = filepath.Join(tmpDir, "depot")
		containerDir := filepath.Join(depotDir, handle)
		Expect(os.MkdirAll(containerDir, 0700)).To(Succeed())
		// By this point, these files are already bind mounted therefore already exist
		Expect(touchFile(filepath.Join(containerDir, "hosts"))).To(Succeed())
		Expect(touchFile(filepath.Join(containerDir, "resolv.conf"))).To(Succeed())

		dnsResolv = &kawasaki.ResolvConfigurer{
			HostsFileCompiler: fakeHostsFileCompiler,
			ResolvCompiler:    fakeResolvCompiler,
			ResolvFilePath:    resolvFilePath,
			DepotDir:          depotDir,
		}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	It("passes additional host entries to the hosts compiler", func() {
		networkConfig := kawasaki.NetworkConfig{
			ContainerHandle:       handle,
			AdditionalHostEntries: []string{"1.2.3.4 foo", "2.3.4.5 bar"},
		}

		Expect(dnsResolv.Configure(log, networkConfig, 42)).To(Succeed())
		_, _, _, _, additionalHostEntries := fakeHostsFileCompiler.CompileArgsForCall(0)
		Expect(additionalHostEntries).To(Equal([]string{
			"1.2.3.4 foo",
			"2.3.4.5 bar",
		}))
	})

	It("passes ip and handle to the hosts compiler", func() {
		networkConfig := kawasaki.NetworkConfig{
			ContainerHandle: handle,
			ContainerIP:     net.IP("10.0.0.1"),
		}

		Expect(dnsResolv.Configure(log, networkConfig, 42)).To(Succeed())
		_, ip, _, handle, _ := fakeHostsFileCompiler.CompileArgsForCall(0)
		Expect(ip).To(Equal(networkConfig.ContainerIP))
		Expect(handle).To(Equal(networkConfig.ContainerHandle))
	})

	It("should write the compiled hosts file in the depot dir", func() {
		compiledHostsFile := "Hello world of hosts"
		fakeHostsFileCompiler.CompileReturns([]byte(compiledHostsFile), nil)

		Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{ContainerHandle: handle}, 42)).To(Succeed())

		Expect(fakeHostsFileCompiler.CompileCallCount()).To(Equal(1))
		hostsFileContents, err := os.ReadFile(filepath.Join(depotDir, handle, "hosts"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(hostsFileContents)).To(Equal(compiledHostsFile))
	})

	Context("when compiling the hosts file fails", func() {
		It("should return an error", func() {
			fakeHostsFileCompiler.CompileReturns(nil, errors.New("banana error"))

			Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{ContainerHandle: handle}, 42)).To(MatchError("banana error"))
		})
	})

	It("should write the container's resolv file in the depot dir", func() {
		fakeResolvCompiler.DetermineReturns([]string{"arbitrary", "lines of text"})

		cfg := kawasaki.NetworkConfig{
			ContainerHandle:       handle,
			BridgeIP:              net.ParseIP("10.11.12.13"),
			OperatorNameservers:   []net.IP{net.ParseIP("9.8.7.6"), net.ParseIP("5.4.3.2")},
			AdditionalNameservers: []net.IP{net.ParseIP("11.11.11.11")},
			PluginNameservers:     []net.IP{net.ParseIP("11.11.11.12")},
			PluginSearchDomains:   []string{"one", "two"},
		}
		Expect(dnsResolv.Configure(log, cfg, 42)).To(Succeed())

		Expect(fakeResolvCompiler.DetermineCallCount()).To(Equal(1))
		actualResolvFileContents, actualHostIP, actualPluginNameservers, actualOperatorNameservers, actualAdditionalNameservers, actualPluginSearchDomains := fakeResolvCompiler.DetermineArgsForCall(0)
		Expect(actualResolvFileContents).To(Equal("nameserver 1.2.3.4\n"))
		Expect(actualHostIP).To(Equal(net.ParseIP("10.11.12.13")))
		Expect(actualPluginNameservers).To(Equal([]net.IP{net.ParseIP("11.11.11.12")}))
		Expect(actualOperatorNameservers).To(Equal([]net.IP{net.ParseIP("9.8.7.6"), net.ParseIP("5.4.3.2")}))
		Expect(actualAdditionalNameservers).To(Equal([]net.IP{net.ParseIP("11.11.11.11")}))
		Expect(actualPluginSearchDomains).To(ConsistOf("one", "two"))

		resolvFileContents, err := os.ReadFile(filepath.Join(depotDir, handle, "resolv.conf"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(resolvFileContents)).To(Equal("arbitrary\nlines of text\n"))
	})

	Describe("files that should already exist not existing", func() {
		Context("and it is the /etc/hosts", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(depotDir, handle, "hosts"))).To(Succeed())
			})

			It("should return an error", func() {
				Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{ContainerHandle: handle}, 42)).To(BeAssignableToTypeOf(&os.PathError{}))
			})
		})

		Context("and it is the /etc/resolv.conf", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(depotDir, handle, "resolv.conf"))).To(Succeed())
			})

			It("should return an error", func() {
				Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{ContainerHandle: handle}, 42)).To(BeAssignableToTypeOf(&os.PathError{}))
			})
		})
	})

	Context("when ipv6 is provided", func() {
		var networkConfig kawasaki.NetworkConfig
		BeforeEach(func() {
			networkConfig = kawasaki.NetworkConfig{
				ContainerHandle: handle,
				ContainerIPv6:   net.ParseIP("2001:db8::1"),
			}
		})

		It("passes ip and handle to the hosts compiler", func() {
			Expect(dnsResolv.Configure(log, networkConfig, 42)).To(Succeed())
			_, _, ipv6, handle, _ := fakeHostsFileCompiler.CompileArgsForCall(0)
			Expect(ipv6).To(Equal(networkConfig.ContainerIPv6))
			Expect(handle).To(Equal(networkConfig.ContainerHandle))
		})
	})
})

func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return file.Close()
}

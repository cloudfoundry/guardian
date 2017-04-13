package kawasaki_test

import (
	"errors"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/kawasaki"
	fakes "code.cloudfoundry.org/guardian/kawasaki/kawasakifakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
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
		tmpDir, err = ioutil.TempDir("", "resolv-test")
		Expect(err).NotTo(HaveOccurred())
		resolvFilePath := filepath.Join(tmpDir, "host-resolv.conf")
		Expect(ioutil.WriteFile(resolvFilePath, []byte("nameserver 1.2.3.4\n"), 0755)).To(Succeed())
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

	It("should write the compiled hosts file in the depot dir", func() {
		compiledHostsFile := "Hello world of hosts"
		fakeHostsFileCompiler.CompileReturns([]byte(compiledHostsFile), nil)

		Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{ContainerHandle: handle}, 42)).To(Succeed())

		Expect(fakeHostsFileCompiler.CompileCallCount()).To(Equal(1))
		hostsFileContents, err := ioutil.ReadFile(filepath.Join(depotDir, handle, "hosts"))
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
		}
		Expect(dnsResolv.Configure(log, cfg, 42)).To(Succeed())

		Expect(fakeResolvCompiler.DetermineCallCount()).To(Equal(1))
		actualResolvFileContents, actualHostIP, actualPluginNameservers, actualOperatorNameservers, actualAdditionalNameservers := fakeResolvCompiler.DetermineArgsForCall(0)
		Expect(actualResolvFileContents).To(Equal("nameserver 1.2.3.4\n"))
		Expect(actualHostIP).To(Equal(net.ParseIP("10.11.12.13")))
		Expect(actualPluginNameservers).To(Equal([]net.IP{net.ParseIP("11.11.11.12")}))
		Expect(actualOperatorNameservers).To(Equal([]net.IP{net.ParseIP("9.8.7.6"), net.ParseIP("5.4.3.2")}))
		Expect(actualAdditionalNameservers).To(Equal([]net.IP{net.ParseIP("11.11.11.11")}))

		resolvFileContents, err := ioutil.ReadFile(filepath.Join(depotDir, handle, "resolv.conf"))
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
})

func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	return file.Close()
}

package kawasaki_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"code.cloudfoundry.org/guardian/kawasaki"
	fakes "code.cloudfoundry.org/guardian/kawasaki/kawasakifakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResolvConfigurer", func() {
	var (
		log                       lager.Logger
		fakeHostsFileCompiler     *fakes.FakeHostFileCompiler
		fakeNameserversDeterminer *fakes.FakeNameserversDeterminer
		fakeNameserversSerializer *fakes.FakeNameserversSerializer
		fakeFileWriter            *fakes.FakeFileWriter
		fakeIdMapReader           *fakes.FakeIdMapReader
		resolvFilePath            string

		dnsResolv *kawasaki.ResolvConfigurer
	)

	BeforeEach(func() {
		log = lagertest.NewTestLogger("test")
		fakeHostsFileCompiler = new(fakes.FakeHostFileCompiler)
		fakeNameserversDeterminer = new(fakes.FakeNameserversDeterminer)
		fakeNameserversSerializer = new(fakes.FakeNameserversSerializer)
		fakeIdMapReader = new(fakes.FakeIdMapReader)
		fakeFileWriter = new(fakes.FakeFileWriter)

		resolvFile, err := ioutil.TempFile("", "resolv-tests")
		Expect(err).NotTo(HaveOccurred())
		defer resolvFile.Close()
		resolvFilePath = resolvFile.Name()
		fmt.Fprintln(resolvFile, "nameserver 1.2.3.4")

		dnsResolv = &kawasaki.ResolvConfigurer{
			HostsFileCompiler:     fakeHostsFileCompiler,
			NameserversDeterminer: fakeNameserversDeterminer,
			NameserversSerializer: fakeNameserversSerializer,
			ResolvFilePath:        resolvFilePath,
			FileWriter:            fakeFileWriter,
			IDMapReader:           fakeIdMapReader,
		}
	})

	AfterEach(func() {
		Expect(os.Remove(resolvFilePath)).To(Succeed())
	})

	It("should write the compiled hosts file", func() {
		compiledHostsFile := []byte("Hello world of hosts")
		fakeHostsFileCompiler.CompileReturns(compiledHostsFile, nil)
		fakeIdMapReader.ReadRootIdReturns(13, nil)

		Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(Succeed())

		Expect(fakeHostsFileCompiler.CompileCallCount()).To(Equal(1))
		_, filePath, contents, rootfsPath, uid, gid := fakeFileWriter.WriteFileArgsForCall(0)
		Expect(filePath).To(Equal("/etc/hosts"))
		Expect(contents).To(Equal(compiledHostsFile))
		Expect(rootfsPath).To(Equal("/proc/42/root"))
		Expect(uid).To(Equal(13))
		Expect(gid).To(Equal(13))
	})

	Context("when compiling the hosts file fails", func() {
		It("should return an error", func() {
			fakeHostsFileCompiler.CompileReturns(nil, errors.New("banana error"))

			Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("banana error"))
		})
	})

	It("should write the container's resolv file", func() {
		compiledResolvFile := []byte("Hello world of resolv.conf")
		fakeNameserversDeterminer.DetermineReturns([]net.IP{net.ParseIP("5.6.7.8")})
		fakeNameserversSerializer.SerializeReturns(compiledResolvFile)
		fakeIdMapReader.ReadRootIdReturns(13, nil)

		cfg := kawasaki.NetworkConfig{
			BridgeIP:              net.ParseIP("10.11.12.13"),
			OperatorNameservers:   []net.IP{net.ParseIP("9.8.7.6"), net.ParseIP("5.4.3.2")},
			AdditionalNameservers: []net.IP{net.ParseIP("11.11.11.11")},
			PluginNameservers:     []net.IP{net.ParseIP("11.11.11.12")},
		}
		Expect(dnsResolv.Configure(log, cfg, 42)).To(Succeed())

		Expect(fakeNameserversDeterminer.DetermineCallCount()).To(Equal(1))
		actualResolvFileContents, actualHostIP, actualPluginNameservers, actualOperatorNameservers, actualAdditionalNameservers := fakeNameserversDeterminer.DetermineArgsForCall(0)
		Expect(actualResolvFileContents).To(Equal("nameserver 1.2.3.4\n"))
		Expect(actualHostIP).To(Equal(net.ParseIP("10.11.12.13")))
		Expect(actualPluginNameservers).To(Equal([]net.IP{net.ParseIP("11.11.11.12")}))
		Expect(actualOperatorNameservers).To(Equal([]net.IP{net.ParseIP("9.8.7.6"), net.ParseIP("5.4.3.2")}))
		Expect(actualAdditionalNameservers).To(Equal([]net.IP{net.ParseIP("11.11.11.11")}))
		Expect(fakeNameserversSerializer.SerializeCallCount()).To(Equal(1))
		containerResolvContents := fakeNameserversSerializer.SerializeArgsForCall(0)
		Expect(containerResolvContents).To(Equal([]net.IP{net.ParseIP("5.6.7.8")}))

		_, filePath, contents, rootfs, uid, gid := fakeFileWriter.WriteFileArgsForCall(1)
		Expect(filePath).To(Equal("/etc/resolv.conf"))
		Expect(contents).To(Equal(compiledResolvFile))
		Expect(rootfs).To(Equal("/proc/42/root"))
		Expect(uid).To(Equal(13))
		Expect(gid).To(Equal(13))
	})

	Context("when reading the uid/gid mapping fails", func() {
		It("returns the error", func() {
			fakeIdMapReader.ReadRootIdReturns(-1, errors.New("boom"))
			Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("boom"))
		})
	})

	Context("when writing a file fails", func() {
		var failedFilePath string

		JustBeforeEach(func() {
			fakeFileWriter.WriteFileStub = func(_ lager.Logger, filePath string, _ []byte, _ string, _, _ int) error {
				if filePath == failedFilePath {
					return errors.New("banana write error")
				}

				return nil
			}
		})

		Context("and it is the /etc/hosts", func() {
			BeforeEach(func() {
				failedFilePath = "/etc/hosts"
			})

			It("should return an error", func() {
				Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("writing file '/etc/hosts': banana write error"))
			})
		})

		Context("and it is the /etc/resolv.conf", func() {
			BeforeEach(func() {
				failedFilePath = "/etc/resolv.conf"
			})

			It("should return an error", func() {
				Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("writing file '/etc/resolv.conf': banana write error"))
			})
		})
	})
})

var _ = Describe("IdMapReader", func() {
	Describe("readId", func() {
		var (
			mappings              []byte
			idMapReader           kawasaki.IdMapReader
			testIdMappingFileName string
		)

		BeforeEach(func() {
			mappings = []byte(`
					1       1001          1
				  0       1000          1
					2       1002          1
				`)
		})

		JustBeforeEach(func() {
			idMapReader = &kawasaki.RootIdMapReader{}

			testIdMappingFile, err := ioutil.TempFile("", "fakeMappings")
			testIdMappingFileName = testIdMappingFile.Name()
			Expect(err).NotTo(HaveOccurred())

			_, err = testIdMappingFile.Write(mappings)
			Expect(err).NotTo(HaveOccurred())
			Expect(testIdMappingFile.Close()).To(Succeed())
		})

		AfterEach(func() {
			os.Remove(testIdMappingFileName)
		})

		Context("when the file cannot be opened", func() {
			It("errors", func() {
				_, err := idMapReader.ReadRootId("blah")
				Expect(err.Error()).To(ContainSubstring("no such file or directory"))
			})
		})

		Context("when there is a root id", func() {
			It("reads the root id from the given path", func() {
				id, err := idMapReader.ReadRootId(testIdMappingFileName)
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal(1000))
			})

			Context("when the mapped id is not a number", func() {
				BeforeEach(func() {
					mappings = []byte(`
				  0       NaN          1
				`)
				})

				It("erorrs", func() {
					_, err := idMapReader.ReadRootId(testIdMappingFileName)
					Expect(err.Error()).To(ContainSubstring("invalid syntax"))
				})
			})
		})

		Context("when there is no root id", func() {
			BeforeEach(func() {
				mappings = []byte(`
					1       1001          1
					2       1002          1
				`)
			})
			It("errors", func() {
				_, err := idMapReader.ReadRootId(testIdMappingFileName)
				Expect(err.Error()).To(ContainSubstring("no root mapping"))
			})
		})
	})
})

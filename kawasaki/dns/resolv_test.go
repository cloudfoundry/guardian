package dns_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/dns"
	fakes "github.com/cloudfoundry-incubator/guardian/kawasaki/dns/dnsfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("ResolvConfigurer", func() {
	var (
		log                    lager.Logger
		fakeHostsFileCompiler  *fakes.FakeCompiler
		fakeResolvFileCompiler *fakes.FakeCompiler
		fakeFileWriter         *fakes.FakeFileWriter

		dnsResolv *dns.ResolvConfigurer
	)

	BeforeEach(func() {
		log = lagertest.NewTestLogger("test")
		fakeHostsFileCompiler = new(fakes.FakeCompiler)
		fakeResolvFileCompiler = new(fakes.FakeCompiler)
		fakeFileWriter = new(fakes.FakeFileWriter)

		dnsResolv = &dns.ResolvConfigurer{
			HostsFileCompiler:  fakeHostsFileCompiler,
			ResolvFileCompiler: fakeResolvFileCompiler,
			FileWriter:         fakeFileWriter,
		}
	})

	It("should write the compiled hosts file", func() {
		compiledHostsFile := []byte("Hello world of hosts")
		fakeHostsFileCompiler.CompileReturns(compiledHostsFile, nil)

		Expect(dnsResolv.Configure(log)).To(Succeed())

		Expect(fakeHostsFileCompiler.CompileCallCount()).To(Equal(1))
		_, filePath, contents := fakeFileWriter.WriteFileArgsForCall(0)
		Expect(filePath).To(Equal("/etc/hosts"))
		Expect(contents).To(Equal(compiledHostsFile))
	})

	Context("when compiling the hosts file fails", func() {
		It("should return an error", func() {
			fakeHostsFileCompiler.CompileReturns(nil, errors.New("banana error"))

			Expect(dnsResolv.Configure(log)).To(MatchError("banana error"))
		})
	})

	It("should write the compile resolv file", func() {
		compiledResolvFile := []byte("Hello world of resolv.conf")
		fakeResolvFileCompiler.CompileReturns(compiledResolvFile, nil)

		Expect(dnsResolv.Configure(log)).To(Succeed())

		Expect(fakeResolvFileCompiler.CompileCallCount()).To(Equal(1))
		_, filePath, contents := fakeFileWriter.WriteFileArgsForCall(1)
		Expect(filePath).To(Equal("/etc/resolv.conf"))
		Expect(contents).To(Equal(compiledResolvFile))
	})

	Context("when compiling the resolv.conf file fails", func() {
		It("should return an error", func() {
			fakeResolvFileCompiler.CompileReturns(nil, errors.New("banana error"))

			Expect(dnsResolv.Configure(log)).To(MatchError("banana error"))
		})
	})

	Context("when writting a file fails", func() {
		var failedFilePath string

		JustBeforeEach(func() {
			fakeFileWriter.WriteFileStub = func(_ lager.Logger, filePath string, _ []byte) error {
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
				Expect(dnsResolv.Configure(log)).To(MatchError("writting file '/etc/hosts': banana write error"))
			})
		})

		Context("and it is the /etc/resolv.conf", func() {
			BeforeEach(func() {
				failedFilePath = "/etc/resolv.conf"
			})

			It("should return an error", func() {
				Expect(dnsResolv.Configure(log)).To(MatchError("writting file '/etc/resolv.conf': banana write error"))
			})
		})
	})
})

var _ = Describe("IdMapReader", func() {
	Describe("readId", func() {
		var (
			mappings              []byte
			idMapReader           dns.RootIdMapReader
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
			idMapReader = dns.RootIdMapReader{}

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

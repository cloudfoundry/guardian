package kawasaki_test

import (
	"errors"
	"io/ioutil"
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
		log                    lager.Logger
		fakeHostsFileCompiler  *fakes.FakeHostFileCompiler
		fakeResolvFileCompiler *fakes.FakeResolvFileCompiler
		fakeFileWriter         *fakes.FakeFileWriter
		fakeIdMapReader        *fakes.FakeIdMapReader

		dnsResolv *kawasaki.ResolvConfigurer
	)

	BeforeEach(func() {
		log = lagertest.NewTestLogger("test")
		fakeHostsFileCompiler = new(fakes.FakeHostFileCompiler)
		fakeResolvFileCompiler = new(fakes.FakeResolvFileCompiler)
		fakeIdMapReader = new(fakes.FakeIdMapReader)
		fakeFileWriter = new(fakes.FakeFileWriter)

		dnsResolv = &kawasaki.ResolvConfigurer{
			HostsFileCompiler:  fakeHostsFileCompiler,
			ResolvFileCompiler: fakeResolvFileCompiler,
			FileWriter:         fakeFileWriter,
			IDMapReader:        fakeIdMapReader,
		}
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

	It("should write the compile resolv file", func() {
		compiledResolvFile := []byte("Hello world of resolv.conf")
		fakeResolvFileCompiler.CompileReturns(compiledResolvFile, nil)
		fakeIdMapReader.ReadRootIdReturns(13, nil)

		Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(Succeed())

		Expect(fakeResolvFileCompiler.CompileCallCount()).To(Equal(1))
		_, filePath, contents, rootfs, uid, gid := fakeFileWriter.WriteFileArgsForCall(1)
		Expect(filePath).To(Equal("/etc/resolv.conf"))
		Expect(contents).To(Equal(compiledResolvFile))
		Expect(rootfs).To(Equal("/proc/42/root"))
		Expect(uid).To(Equal(13))
		Expect(gid).To(Equal(13))
	})

	Context("when compiling the resolv.conf file fails", func() {
		It("should return an error", func() {
			fakeResolvFileCompiler.CompileReturns(nil, errors.New("banana error"))

			Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("banana error"))
		})
	})

	Context("when reading the uid/gid mapping fails", func() {
		It("returns the error", func() {
			fakeIdMapReader.ReadRootIdReturns(-1, errors.New("boom"))
			Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("boom"))
		})
	})

	Context("when writting a file fails", func() {
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
				Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("writting file '/etc/hosts': banana write error"))
			})
		})

		Context("and it is the /etc/resolv.conf", func() {
			BeforeEach(func() {
				failedFilePath = "/etc/resolv.conf"
			})

			It("should return an error", func() {
				Expect(dnsResolv.Configure(log, kawasaki.NetworkConfig{}, 42)).To(MatchError("writting file '/etc/resolv.conf': banana write error"))
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

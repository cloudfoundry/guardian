package unpacker_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/unpacker"
	"code.cloudfoundry.org/grootfs/cloner/unpacker/unpackerfakes"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

func init() {
	if os.Args[1] == "untar" {
		fakeIDMapper := new(unpackerfakes.FakeIDMapper)
		tarUnpacker := unpacker.NewTarUnpacker(fakeIDMapper)

		logger := lagertest.NewTestLogger("test-tar-cloner-untar")
		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")

		rootFSPath := os.Args[2]
		if err := tarUnpacker.Untar(logger, ctrlPipeR, os.Stdin, rootFSPath); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		os.Exit(0)
	}
}

var _ = Describe("TarUnpacker", func() {
	var (
		logger lager.Logger

		bundleDir  string
		rootFSPath string

		fakeIDMapper *unpackerfakes.FakeIDMapper
		tarUnpacker  *unpacker.TarUnpacker

		stream *gbytes.Buffer
	)

	BeforeEach(func() {
		var err error

		bundleDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		rootFSPath = path.Join(bundleDir, "rootfs")

		fakeIDMapper = new(unpackerfakes.FakeIDMapper)
		tarUnpacker = unpacker.NewTarUnpacker(fakeIDMapper)

		logger = lagertest.NewTestLogger("test-graph")

		tempDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(tempDir, "a_file"), []byte("hello-world"), 0600)).To(Succeed())

		stream = gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("tar", "-c", "-C", tempDir, "."), stream, nil)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundleDir)).To(Succeed())
	})

	Describe("Unpack", func() {
		It("does write the image contents in the rootfs directory", func() {
			Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
				Stream:     stream,
				RootFSPath: rootFSPath,
			})).To(Succeed())

			filePath := path.Join(rootFSPath, "a_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := ioutil.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))
		})

		Context("when using mapping users", func() {
			Describe("UIDMappings", func() {
				It("uses the provided uid mapping", func() {
					Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
						Stream:     stream,
						RootFSPath: rootFSPath,
						UIDMappings: []groot.IDMappingSpec{
							groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
						},
					})).To(Succeed())

					Expect(fakeIDMapper.MapUIDsCallCount()).To(Equal(1))
					_, _, mappings := fakeIDMapper.MapUIDsArgsForCall(0)

					Expect(mappings).To(Equal([]groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
					}))
				})

				Context("when mapping fails", func() {
					BeforeEach(func() {
						fakeIDMapper.MapUIDsReturns(errors.New("Boom!"))
					})

					It("returns an error", func() {
						Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
							Stream:     stream,
							RootFSPath: rootFSPath,
							UIDMappings: []groot.IDMappingSpec{
								groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
							},
						})).To(MatchError(ContainSubstring("Boom!")))
					})
				})
			})

			Describe("GIDMappings", func() {
				It("uses the provided gid mapping", func() {
					Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
						Stream:     stream,
						RootFSPath: rootFSPath,
						GIDMappings: []groot.IDMappingSpec{
							groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
						},
					})).To(Succeed())

					Expect(fakeIDMapper.MapGIDsCallCount()).To(Equal(1))
					_, _, mappings := fakeIDMapper.MapGIDsArgsForCall(0)

					Expect(mappings).To(Equal([]groot.IDMappingSpec{
						groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
					}))
				})

				Context("when mapping fails", func() {
					BeforeEach(func() {
						fakeIDMapper.MapGIDsReturns(errors.New("Boom!"))
					})

					It("returns an error", func() {
						Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
							Stream:     stream,
							RootFSPath: rootFSPath,
							GIDMappings: []groot.IDMappingSpec{
								groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
							},
						})).To(MatchError(ContainSubstring("Boom!")))
					})
				})
			})
		})

		Context("when it fails to untar", func() {
			BeforeEach(func() {
				stream = gbytes.NewBuffer()
				stream.Write([]byte("not-a-tar"))
			})

			It("returns an error", func() {
				Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
					Stream:     stream,
					RootFSPath: rootFSPath,
				})).To(
					MatchError(ContainSubstring(fmt.Sprintf("writing to `%s`", rootFSPath))),
				)
			})

			It("returns the command output", func() {
				Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
					Stream:     stream,
					RootFSPath: rootFSPath,
				})).To(
					MatchError(ContainSubstring("This does not look like a tar")),
				)
			})
		})

		Context("when creating the target directory fails", func() {
			It("returns an error", func() {
				err := tarUnpacker.Unpack(logger, cloner.UnpackSpec{
					Stream:     stream,
					RootFSPath: "/tmp/some-destination/bundles/1000",
				})

				Expect(err).To(MatchError(ContainSubstring("making destination directory")))
			})
		})
	})

	Describe("Untar", func() {
		var (
			ctrlPipeR io.ReadCloser
			ctrlPipeW io.WriteCloser
		)

		BeforeEach(func() {
			var err error

			Expect(os.Mkdir(rootFSPath, 0755)).To(Succeed())

			ctrlPipeR, ctrlPipeW, err = os.Pipe()
			Expect(err).NotTo(HaveOccurred())
		})

		It("untars given an input stream", func() {
			_, err := ctrlPipeW.Write([]byte{0})
			Expect(err).NotTo(HaveOccurred())

			Expect(tarUnpacker.Untar(logger, ctrlPipeR, stream, rootFSPath)).To(Succeed())
			Expect(path.Join(rootFSPath, "a_file")).To(BeARegularFile())
		})

		It("waits for the control pipe to continue", func() {
			untarFinishedChan := make(chan struct{})

			go func() {
				defer GinkgoRecover()
				Expect(tarUnpacker.Untar(logger, ctrlPipeR, stream, rootFSPath)).To(Succeed())
				close(untarFinishedChan)
			}()

			Consistently(untarFinishedChan).ShouldNot(BeClosed())

			_, err := ctrlPipeW.Write([]byte{0})
			Expect(err).NotTo(HaveOccurred())

			Eventually(untarFinishedChan).Should(BeClosed())
		})

		Context("when the control pipe gets closed", func() {
			It("silently bails out", func() {
				Expect(ctrlPipeW.Close()).To(Succeed())

				emptyBuffer := gbytes.NewBuffer()
				Expect(tarUnpacker.Untar(logger, ctrlPipeR, emptyBuffer, rootFSPath)).To(Succeed())
			})
		})

		Context("when untar fails", func() {
			var emptyBuffer *gbytes.Buffer

			BeforeEach(func() {
				_, err := ctrlPipeW.Write([]byte{0})
				Expect(err).NotTo(HaveOccurred())
				emptyBuffer = gbytes.NewBuffer()
			})

			It("returns an error", func() {
				emptyBuffer := gbytes.NewBuffer()
				Expect(tarUnpacker.Untar(logger, ctrlPipeR, emptyBuffer, rootFSPath)).To(MatchError(ContainSubstring("untaring")))
			})

			It("forwards tar's output", func() {
				emptyBuffer := gbytes.NewBuffer()
				Expect(tarUnpacker.Untar(logger, ctrlPipeR, emptyBuffer, rootFSPath)).To(MatchError(ContainSubstring("This does not look like a tar archive")))
			})
		})
	})
})

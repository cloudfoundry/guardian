package cloner_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/clonerfakes"
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
		fakeIDMapper := new(clonerfakes.FakeIDMapper)
		tarCloner := cloner.NewTarCloner(fakeIDMapper)

		logger := lagertest.NewTestLogger("test-tar-cloner-untar")
		ctrlPipeR := os.NewFile(3, "/ctrl/pipe")

		toDir := os.Args[2]
		if path.Base(toDir) == "fail-to-untar" {
			fmt.Fprintf(os.Stdout, "failed to untar")
			os.Exit(1)
		}

		if err := tarCloner.Untar(logger, ctrlPipeR, os.Stdin, toDir); err != nil {
			os.Exit(1)
		}

		os.Exit(0)
	}
}

var _ = Describe("TarCloner", func() {
	var (
		logger lager.Logger

		bundleDir string
		toDir     string

		fakeIDMapper *clonerfakes.FakeIDMapper
		tarCloner    *cloner.TarCloner
	)

	BeforeEach(func() {
		var err error

		bundleDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		toDir = path.Join(bundleDir, "rootfs")

		fakeIDMapper = new(clonerfakes.FakeIDMapper)
		tarCloner = cloner.NewTarCloner(fakeIDMapper)

		logger = lagertest.NewTestLogger("test-graph")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundleDir)).To(Succeed())
	})

	Describe("Clone", func() {
		var fromDir string

		BeforeEach(func() {
			var err error

			fromDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(path.Join(fromDir, "a_file"), []byte("hello-world"), 0600)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(fromDir)).To(Succeed())
		})

		It("does have the image contents in the rootfs directory", func() {
			Expect(tarCloner.Clone(logger, groot.CloneSpec{
				FromDir: fromDir,
				ToDir:   toDir,
			})).To(Succeed())

			filePath := path.Join(toDir, "a_file")
			Expect(filePath).To(BeARegularFile())
			contents, err := ioutil.ReadFile(filePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))
		})

		Context("when using mapping users", func() {
			Describe("UIDMappings", func() {
				It("uses the uid provided", func() {
					Expect(tarCloner.Clone(logger, groot.CloneSpec{
						FromDir: fromDir,
						ToDir:   toDir,
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

				Context("when it fails", func() {
					BeforeEach(func() {
						fakeIDMapper.MapUIDsReturns(errors.New("Boom!"))
					})

					It("returns an error", func() {
						Expect(tarCloner.Clone(logger, groot.CloneSpec{
							FromDir: fromDir,
							ToDir:   toDir,
							UIDMappings: []groot.IDMappingSpec{
								groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
							},
						})).To(MatchError(ContainSubstring("Boom!")))
					})
				})
			})

			Describe("GIDMappings", func() {
				It("uses the uid provided", func() {
					Expect(tarCloner.Clone(logger, groot.CloneSpec{
						FromDir: fromDir,
						ToDir:   toDir,
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

				Context("when it fails", func() {
					BeforeEach(func() {
						fakeIDMapper.MapGIDsReturns(errors.New("Boom!"))
					})

					It("returns an error", func() {
						Expect(tarCloner.Clone(logger, groot.CloneSpec{
							FromDir: fromDir,
							ToDir:   toDir,
							GIDMappings: []groot.IDMappingSpec{
								groot.IDMappingSpec{HostID: 1000, NamespaceID: 2000, Size: 10},
							},
						})).To(MatchError(ContainSubstring("Boom!")))
					})
				})
			})
		})

		Context("when the image path does not exist", func() {
			It("returns an error", func() {
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: "/does/not/exist",
					ToDir:   toDir,
				})).To(
					MatchError(ContainSubstring("image path `/does/not/exist` was not found")),
				)
			})
		})

		Context("when the image contains files that can only be read by root", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(path.Join(fromDir, "a-file"), []byte("hello-world"), 0000)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
				})).To(
					MatchError(ContainSubstring(fmt.Sprintf("reading from `%s`", fromDir))),
				)
			})

			It("forwards tar's stderr", func() {
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
				})).To(
					MatchError(ContainSubstring("Permission denied")),
				)
			})
		})

		Context("when untarring fails for reasons", func() {
			It("returns an error", func() {
				toDir := path.Join(bundleDir, "fail-to-untar")
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
				})).To(
					MatchError(ContainSubstring(fmt.Sprintf("writing to `%s`", toDir))),
				)
			})

			It("returns the command output", func() {
				toDir := path.Join(bundleDir, "fail-to-untar")
				Expect(tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   toDir,
				})).To(
					MatchError(ContainSubstring("failed to untar")),
				)
			})
		})

		Context("when creating the target directory fails", func() {
			It("returns an error", func() {
				err := tarCloner.Clone(logger, groot.CloneSpec{
					FromDir: fromDir,
					ToDir:   "/tmp/some-destination/bundles/1000",
				})

				Expect(err).To(MatchError(ContainSubstring("making destination directory")))
			})
		})
	})

	Describe("Untar", func() {
		var (
			buffer    *gbytes.Buffer
			ctrlPipeR io.ReadCloser
			ctrlPipeW io.WriteCloser
		)

		BeforeEach(func() {
			var err error

			tempDir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(ioutil.WriteFile(path.Join(tempDir, "a_file"), []byte("hello-world"), 0600)).To(Succeed())

			buffer = gbytes.NewBuffer()
			sess, err := gexec.Start(exec.Command("tar", "-c", "-C", tempDir, "."), buffer, nil)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			Expect(os.Mkdir(toDir, 0755)).To(Succeed())

			ctrlPipeR, ctrlPipeW, err = os.Pipe()
			Expect(err).NotTo(HaveOccurred())
		})

		It("untars given an input stream", func() {
			_, err := ctrlPipeW.Write([]byte{0})
			Expect(err).NotTo(HaveOccurred())

			Expect(tarCloner.Untar(logger, ctrlPipeR, buffer, toDir)).To(Succeed())
			Expect(path.Join(toDir, "a_file")).To(BeARegularFile())
		})

		It("waits for the control pipe to continue", func() {
			untarFinishedChan := make(chan struct{})

			go func() {
				defer GinkgoRecover()
				Expect(tarCloner.Untar(logger, ctrlPipeR, buffer, toDir)).To(Succeed())
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
				Expect(tarCloner.Untar(logger, ctrlPipeR, emptyBuffer, toDir)).To(Succeed())
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
				Expect(tarCloner.Untar(logger, ctrlPipeR, emptyBuffer, toDir)).To(MatchError(ContainSubstring("untaring")))
			})

			It("forwards tar's output", func() {
				emptyBuffer := gbytes.NewBuffer()
				Expect(tarCloner.Untar(logger, ctrlPipeR, emptyBuffer, toDir)).To(MatchError(ContainSubstring("This does not look like a tar archive")))
			})
		})
	})
})

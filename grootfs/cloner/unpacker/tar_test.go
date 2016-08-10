package unpacker_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/cloner"
	"code.cloudfoundry.org/grootfs/cloner/unpacker"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Tar", func() {
	var (
		logger lager.Logger

		bundleDir  string
		rootFSPath string

		tarUnpacker *unpacker.TarUnpacker

		stream *gbytes.Buffer
	)

	BeforeEach(func() {
		var err error

		bundleDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		rootFSPath = path.Join(bundleDir, "rootfs")

		tarUnpacker = unpacker.NewTarUnpacker()

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

	Context("when it fails to untar", func() {
		BeforeEach(func() {
			stream = gbytes.NewBuffer()
			stream.Write([]byte("not-a-tar"))
		})

		It("returns an error", func() {
			Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
				Stream:     stream,
				RootFSPath: rootFSPath,
			})).NotTo(Succeed())
		})

		It("returns the command output", func() {
			Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
				Stream:     stream,
				RootFSPath: rootFSPath,
			})).To(
				MatchError(ContainSubstring("tar:")),
			)
		})
	})

	Context("when creating the target directory fails", func() {
		It("returns an error", func() {
			err := tarUnpacker.Unpack(logger, cloner.UnpackSpec{
				Stream:     stream,
				RootFSPath: "/some-destination/bundles/1000",
			})

			Expect(err).To(MatchError(ContainSubstring("making destination directory")))
		})
	})

	Context("when the target directory exists", func() {
		It("still works", func() {
			Expect(os.Mkdir(rootFSPath, 0770)).To(Succeed())

			Expect(tarUnpacker.Unpack(logger, cloner.UnpackSpec{
				Stream:     stream,
				RootFSPath: rootFSPath,
			})).To(Succeed())
		})
	})
})

package groot_test

import (
	"io/ioutil"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create", func() {

	Context("when `--image` is a local directory", func() {
		var imagePath string

		BeforeEach(func() {
			var err error
			imagePath, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
		})

		It("creates a root filesystem", func() {
			bundle := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "random-id")
			bundleContentPath := path.Join(bundle.RootFSPath(), "foo")
			Expect(bundleContentPath).To(BeARegularFile())
			fooContents, err := ioutil.ReadFile(bundleContentPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(fooContents)).To(Equal("hello-world"))
		})

		Context("when local directory does not exist", func() {
			It("returns an error", func() {
				cmd := exec.Command(GrootFSBin, "--graph", GraphPath, "create", "--image", "/invalid/image")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("when two rootfses are using the same image", func() {
			It("isolates them", func() {
				bundle := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "random-id")
				anotherBundle := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "another-random-id")
				Expect(ioutil.WriteFile(path.Join(bundle.RootFSPath(), "bar"), []byte("hello-world"), 0644)).To(Succeed())
				Expect(path.Join(anotherBundle.RootFSPath(), "bar")).NotTo(BeARegularFile())
			})
		})

		Context("when the id is not provided", func() {
			It("fails", func() {
				cmd := exec.Command(GrootFSBin, "--graph", GraphPath, "create", "--image", imagePath)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
			})
		})

		Context("when the mappings are invalid", func() {
			It("should fail when the uid mapping is invalid", func() {
				cmd := exec.Command(
					GrootFSBin, "--graph", GraphPath,
					"create", "--image", imagePath,
					"--uid-mapping", "1:hello:65000",
					"some-id",
				)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(sess.Wait()).NotTo(gexec.Exit(0))
			})

			It("should fail when the gid mapping is invalid", func() {
				cmd := exec.Command(
					GrootFSBin, "--graph", GraphPath,
					"create", "--image", imagePath,
					"--gid-mapping", "1:groot:65000",
					"some-id",
				)

				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Expect(sess.Wait()).NotTo(gexec.Exit(0))
			})
		})
	})
})

package groot_test

import (
	"io/ioutil"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create", func() {
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

	Context("when the id is already being used", func() {
		BeforeEach(func() {
			Expect(integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "random-id")).NotTo(BeNil())
		})

		It("fails and produces a useful error", func() {
			cmd := exec.Command(GrootFSBin, "--graph", GraphPath, "create", "--image", imagePath, "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Eventually(sess.Out).Should(gbytes.Say("bundle for id `random-id` already exists"))
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})

	Context("when a mappings flag is invalid", func() {
		It("fails when the uid mapping is invalid", func() {
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

		It("fails when the gid mapping is invalid", func() {
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

	Context("when groot does not have permissions to apply the requested mapping", func() {
		It("returns the newuidmap output in the stdout", func() {
			cmd := exec.Command(
				GrootFSBin, "--graph", GraphPath,
				"create", "--image", imagePath,
				"--uid-mapping", "1:1:65000",
				"some-id",
			)

			buffer := gbytes.NewBuffer()
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))

			Eventually(buffer).Should(gbytes.Say(`range [\[\)0-9\-]* -> [\[\)0-9\-]* not allowed`))
		})

		It("does not leak the bundle directory", func() {
			cmd := exec.Command(
				GrootFSBin, "--graph", GraphPath,
				"create", "--image", imagePath,
				"--uid-mapping", "1:1:65000",
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))

			Expect(path.Join(GraphPath, "bundles", "some-id")).ToNot(BeAnExistingFile())
		})
	})

	Context("when `--image` is a remote scheme url", func() {
		Context("when it uses docker registry", func() {
			It("creates a root filesystem based on the image provided", func() {
				imageURL := "docker:///cfgarden/empty:v0.1.0"
				bundle := integration.CreateBundle(GrootFSBin, GraphPath, imageURL, "random-id")

				Expect(path.Join(bundle.RootFSPath(), "hello")).To(BeARegularFile())
			})
		})
	})
})

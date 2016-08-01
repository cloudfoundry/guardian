package groot_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	It("creates a root filesystem given a local directory", func() {
		bundlePath := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "random-id")
		Expect(path.Join(bundlePath, "rootfs", "foo")).To(BeARegularFile())
		fooContents, err := ioutil.ReadFile(path.Join(bundlePath, "rootfs", "foo"))
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
			bundlePath := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "random-id")
			anotherBundlePath := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "another-random-id")
			Expect(ioutil.WriteFile(path.Join(bundlePath, "rootfs", "bar"), []byte("hello-world"), 0644)).To(Succeed())
			Expect(path.Join(anotherBundlePath, "rootfs", "bar")).NotTo(BeARegularFile())
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

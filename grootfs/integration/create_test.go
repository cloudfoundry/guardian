package integration_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create", func() {
	It("creates a root filesystem given a local directory", func() {
		imagePath, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(imagePath)

		Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())

		cmd := exec.Command(GrootFSBin, "--graph", GraphPath, "create", "--image", imagePath)
		sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		bundlePath := strings.TrimSpace(string(sess.Out.Contents()))
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
})

package groot_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	var (
		sourceImagePath string
		baseImagePath   string
		image           groot.Image
	)

	BeforeEach(func() {
		var err error
		sourceImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
		image = integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)
	})

	It("deletes an existing image", func() {
		Expect(Runner.Delete("random-id")).To(Succeed())
		Expect(image.Path).NotTo(BeAnExistingFile())
	})

	It("destroys the quota group associated with the volume", func() {
		rootIDBuffer := gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("sudo", "btrfs", "inspect-internal", "rootid", image.RootFSPath), rootIDBuffer, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
		rootID := strings.TrimSpace(string(rootIDBuffer.Contents()))

		sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", StorePath), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
		Expect(sess).To(gbytes.Say(rootID))

		Expect(Runner.Delete("random-id")).To(Succeed())

		sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", StorePath), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
		Expect(sess).ToNot(gbytes.Say(rootID))
	})

	Context("when the store doesn't exist", func() {
		It("logs the image path", func() {
			logBuffer := gbytes.NewBuffer()
			err := Runner.WithStore("/invalid-store").WithStderr(logBuffer).
				Delete("/path/to/random-id")
			Expect(err).ToNot(HaveOccurred())
			Expect(logBuffer).To(gbytes.Say(`"id":"/path/to/random-id"`))
		})
	})

	Context("when a path is provided instead of an ID", func() {
		It("deletes the image by the path", func() {
			Expect(Runner.Delete(image.Path)).To(Succeed())
			Expect(image.Path).NotTo(BeAnExistingFile())
		})

		Context("when a path to an image does not exist", func() {
			It("succeeds but logs a warning", func() {
				fakePath := path.Join(StorePath, CurrentUserID, "images/non_existing")
				Expect(fakePath).NotTo(BeAnExistingFile())

				cmd := exec.Command(GrootFSBin, "--store", StorePath, "delete", fakePath)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("image `non_existing` was not found"))
			})
		})

		Context("when the path provided doesn't belong to the `--store` provided", func() {
			It("returns an error", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "delete", "/Iamnot/in/the/storage/images/1234/rootfs")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))
				Eventually(sess.Out).Should(gbytes.Say("path `/Iamnot/in/the/storage/images/1234/rootfs` is outside store path"))
			})
		})
	})

	Context("when the image ID doesn't exist", func() {
		It("succeeds but logs a warning", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "delete", "non-existing-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			Eventually(sess.Out).Should(gbytes.Say("image `non-existing-id` was not found"))
		})
	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "delete")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("id was not specified"))
		})
	})

	Context("when drax is not in PATH", func() {
		It("returns a warning", func() {
			cmd := exec.Command(GrootFSBin, "--log-level", "info", "--store", StorePath, "delete", "random-id")
			cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			Eventually(sess.Err).Should(gbytes.Say("could not delete quota group"))
			Eventually(sess.Out).Should(gbytes.Say("Image random-id deleted"))
		})
	})
})

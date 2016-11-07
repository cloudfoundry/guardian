package groot_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Metrics", func() {
	var (
		baseImagePath string
		image    groot.Image
	)

	BeforeEach(func() {
		var err error
		baseImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when image exists", func() {
		BeforeEach(func() {
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(baseImagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			image = integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)
			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(image.RootFSPath, "hello")), "bs=1048576", "count=4")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the last parameter is the image ID", func() {
			It("returns the metrics for given image id", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "metrics", "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(gbytes.Say(`{"disk_usage":{"total_bytes_used":9453568,"exclusive_bytes_used":4210688}}`))
			})
		})

		Context("when the last parameter is the image path", func() {
			It("returns the metrics for given image path", func() {
				cmd := exec.Command(GrootFSBin, "--log-level", "debug", "--store", StorePath, "--drax-bin", DraxBin, "metrics", image.Path)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(gbytes.Say(`{"disk_usage":{"total_bytes_used":9453568,"exclusive_bytes_used":4210688}}`))
			})
		})
	})

	Context("when the image id doesn't exist", func() {
		Context("when the last parameter is a image id", func() {
			It("returns an error", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "metrics", "invalid-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Out).Should(gbytes.Say("No such file or directory"))
			})
		})

		Context("when the last parameter is a path", func() {
			It("returns an error", func() {
				imagePath := filepath.Join(StorePath, CurrentUserID, store.IMAGES_DIR_NAME, "not-here")
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "metrics", imagePath)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Out).Should(gbytes.Say("No such file or directory"))
			})

			Context("when the path provided doesn't belong to the `--store` provided", func() {
				It("returns an error", func() {
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "metrics", "/Iamnot/in/the/storage/images/1234/rootfs")
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(1))
					Eventually(sess.Out).Should(gbytes.Say("path `/Iamnot/in/the/storage/images/1234/rootfs` is outside store path"))
				})
			})
		})
	})

	Context("when the image id is not provided", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "metrics")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("invalid arguments"))
		})
	})
})

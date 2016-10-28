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
		imagePath string
		bundle    groot.Bundle
	)

	BeforeEach(func() {
		var err error
		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when bundle exists", func() {
		BeforeEach(func() {
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(imagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			bundle = integration.CreateBundle(GrootFSBin, StorePath, DraxBin, imagePath, "random-id", 0)
			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(bundle.RootFSPath, "hello")), "bs=1048576", "count=4")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		})

		Context("when the last parameter is the bundle ID", func() {
			It("returns the metrics for given bundle id", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "metrics", "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(gbytes.Say(`{"disk_usage":{"total_bytes_used":9453568,"exclusive_bytes_used":4210688}}`))
			})
		})

		Context("when the last parameter is the bundle path", func() {
			It("returns the metrics for given bundle path", func() {
				cmd := exec.Command(GrootFSBin, "--log-level", "debug", "--store", StorePath, "--drax-bin", DraxBin, "metrics", bundle.Path)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				Eventually(sess.Out).Should(gbytes.Say(`{"disk_usage":{"total_bytes_used":9453568,"exclusive_bytes_used":4210688}}`))
			})
		})
	})

	Context("when the bundle id doesn't exist", func() {
		Context("when the last parameter is a bundle id", func() {
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
				bundlePath := filepath.Join(StorePath, CurrentUserID, store.BUNDLES_DIR_NAME, "not-here")
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "metrics", bundlePath)
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Out).Should(gbytes.Say("No such file or directory"))
			})

			Context("when the path provided doesn't belong to the `--store` provided", func() {
				It("returns an error", func() {
					cmd := exec.Command(GrootFSBin, "--store", StorePath, "metrics", "/Iamnot/in/the/storage/bundles/1234/rootfs")
					sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
					Expect(err).ToNot(HaveOccurred())
					Eventually(sess).Should(gexec.Exit(1))
					Eventually(sess.Out).Should(gbytes.Say("path `/Iamnot/in/the/storage/bundles/1234/rootfs` is outside store path"))
				})
			})
		})
	})

	Context("when the bundle id is not provided", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "--drax-bin", DraxBin, "metrics")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("invalid arguments"))
		})
	})
})

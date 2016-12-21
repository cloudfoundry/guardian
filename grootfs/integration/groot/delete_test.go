package groot_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	var (
		baseImagePath string
		image         groot.Image
	)

	BeforeEach(func() {
		var err error
		baseImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(baseImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
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

	Context("when a path is provided instead of an ID", func() {
		It("deletes the image by the path", func() {
			Expect(Runner.Delete(image.Path)).To(Succeed())
			Expect(image.Path).NotTo(BeAnExistingFile())
		})

		Context("when the path provided doesn't belong to the `--store` provided", func() {
			It("returns an error", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "delete", "/Iamnot/in/the/storage/images/1234/rootfs")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess.Out).Should(gbytes.Say("path `/Iamnot/in/the/storage/images/1234/rootfs` is outside store path"))
			})
		})
	})

	Describe("--config global flag", func() {
		var cfg config.Config

		BeforeEach(func() {
			cfg = config.Config{}
		})

		JustBeforeEach(func() {
			Runner.SetConfig(cfg)
		})

		Describe("store path", func() {
			BeforeEach(func() {
				cfg.BaseStorePath = StorePath
			})

			It("uses the store path from the config file", func() {
				err := Runner.WithoutStore().Delete("random-id")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("drax bin", func() {
			var (
				draxCalledFile *os.File
				draxBin        *os.File
				tempFolder     string
			)

			BeforeEach(func() {
				tempFolder, draxBin, draxCalledFile = integration.CreateFakeDrax()
				cfg.DraxBin = draxBin.Name()
			})

			It("uses the drax bin from the config file", func() {
				Expect(Runner.WithoutDraxBin().Delete("random-id")).To(Succeed())

				contents, err := ioutil.ReadFile(draxCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - drax"))
			})
		})

		Describe("btrfs bin", func() {
			var (
				btrfsCalledFile *os.File
				btrfsBin        *os.File
				tempFolder      string
			)

			BeforeEach(func() {
				tempFolder, btrfsBin, btrfsCalledFile = integration.CreateFakeBin("btrfs")
				cfg.BtrfsBin = btrfsBin.Name()
			})

			It("uses the btrfs bin from the config file", func() {
				err := Runner.Delete("random-id")
				Expect(err).To(HaveOccurred())

				contents, err := ioutil.ReadFile(btrfsCalledFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("I'm groot - btrfs"))
			})
		})
	})

	Context("when the image ID doesn't exist", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "delete", "non-existing-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("image not found"))
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

package groot_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"

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

	Context("when inclusive disk limit is provided", func() {
		It("creates a bundle with supplied limit", func() {
			cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(imagePath, "fatfile")), "bs=1048576", "count=5")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			bundle := integration.CreateBundle(GrootFSBin, StorePath, imagePath, "random-id", int64(10*1024*1024))

			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(bundle.RootFSPath(), "hello")), "bs=1048576", "count=4")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))

			cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(bundle.RootFSPath(), "hello2")), "bs=1048576", "count=2")
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Expect(sess.Err).To(gbytes.Say("Disk quota exceeded"))
		})

		Context("when the disk limit value is invalid", func() {
			It("fails with a helpful error", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--disk-limit-size-bytes", "-200", imagePath, "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Eventually(sess).Should(gbytes.Say("disk limit cannot be negative"))
			})
		})

		Context("when the exclude-image-from-quota is also provided", func() {
			It("creates a bundle with supplied limit, but doesn't take into account the base image size", func() {
				cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--disk-limit-size-bytes", "10485760", "--exclude-image-from-quota", imagePath, "random-id")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				rootfsPath := filepath.Join(StorePath, "bundles/random-id/rootfs")
				cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(rootfsPath, "hello")), "bs=1048576", "count=6")
				sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(0))

				cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(rootfsPath, "hello2")), "bs=1048576", "count=5")
				sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).ToNot(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))
				Expect(sess.Err).To(gbytes.Say("Disk quota exceeded"))
			})
		})
	})

	Context("when no --store option is given", func() {
		It("uses the default store path", func() {
			Expect("/var/lib/grootfs/bundles").ToNot(BeAnExistingFile())

			cmd := exec.Command(GrootFSBin, "create", imagePath, "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			// It will fail at this point, because /var/lib/grootfs doesn't exist
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess).Should(gbytes.Say("making directory `/var/lib/grootfs`"))
		})
	})

	Context("when two rootfses are using the same image", func() {
		It("isolates them", func() {
			bundle := integration.CreateBundle(GrootFSBin, StorePath, imagePath, "random-id", 0)
			anotherBundle := integration.CreateBundle(GrootFSBin, StorePath, imagePath, "another-random-id", 0)
			Expect(ioutil.WriteFile(path.Join(bundle.RootFSPath(), "bar"), []byte("hello-world"), 0644)).To(Succeed())
			Expect(path.Join(anotherBundle.RootFSPath(), "bar")).NotTo(BeARegularFile())
		})
	})

	Context("when the id is already being used", func() {
		BeforeEach(func() {
			Expect(integration.CreateBundle(GrootFSBin, StorePath, imagePath, "random-id", 0)).NotTo(BeNil())
		})

		It("fails and produces a useful error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", imagePath, "random-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Eventually(sess.Out).Should(gbytes.Say("bundle for id `random-id` already exists"))
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})

	Context("when groot does not have permissions to apply the requested mapping", func() {
		It("returns the newuidmap output in the stdout", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create",
				"--uid-mapping", "1:1:65000",
				imagePath,
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
				GrootFSBin, "--store", StorePath,
				"create",
				"--uid-mapping", "1:1:65000",
				imagePath,
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))

			Expect(path.Join(StorePath, "bundles", "some-id")).ToNot(BeAnExistingFile())
		})
	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", imagePath)
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
		})
	})

	Context("when the image is invalid", func() {
		It("fails", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create",
				"*@#%^!&",
				"some-id",
			)

			buffer := gbytes.NewBuffer()
			sess, err := gexec.Start(cmd, buffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).To(gexec.Exit(1))
			Eventually(sess).Should(gbytes.Say("parsing image url: parse"))
			Eventually(sess).Should(gbytes.Say("invalid URL escape"))
		})
	})

	Context("when a mappings flag is invalid", func() {
		It("fails when the uid mapping is invalid", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create", imagePath,
				"--uid-mapping", "1:hello:65000",
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))
		})

		It("fails when the gid mapping is invalid", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"create", imagePath,
				"--gid-mapping", "1:groot:65000",
				"some-id",
			)

			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Expect(sess.Wait()).NotTo(gexec.Exit(0))
		})
	})
})

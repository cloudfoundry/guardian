package unpacker_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Tar unpacker - Linux tests", func() {
	var (
		tarUnpacker   *unpacker.TarUnpacker
		logger        lager.Logger
		baseImagePath string
		stream        *gbytes.Buffer
		targetPath    string
	)

	BeforeEach(func() {
		var err error
		tarUnpacker, err = unpacker.NewTarUnpacker(unpacker.UnpackStrategy{Name: "btrfs"})
		Expect(err).NotTo(HaveOccurred())

		targetPath, err = ioutil.TempDir("", "target-")
		Expect(err).NotTo(HaveOccurred())

		baseImagePath, err = ioutil.TempDir("", "base-image-")
		Expect(err).NotTo(HaveOccurred())

		logger = lagertest.NewTestLogger("test-store")
	})

	JustBeforeEach(func() {
		stream = gbytes.NewBuffer()
		sess, err := gexec.Start(exec.Command("tar", "-c", "-C", baseImagePath, "."), stream, nil)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))
	})

	AfterEach(func() {
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
		Expect(os.RemoveAll(targetPath)).To(Succeed())
	})

	Describe("Devices files", func() {
		BeforeEach(func() {
			Expect(exec.Command("sudo", "mknod", path.Join(baseImagePath, "a_device"), "c", "1", "8").Run()).To(Succeed())
		})

		It("excludes them", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			filePath := path.Join(targetPath, "a_device")
			Expect(filePath).ToNot(BeAnExistingFile())
		})
	})

	Describe("modification time", func() {
		var symlinkModTime time.Time

		setSymlinkModtime := func(symlinkPath string, modTime time.Time) {
			cmd := exec.Command(
				"touch", "-h",
				"-d", modTime.Format("2006-01-02T15:04:05.999999999 -0700"),
				symlinkPath,
			)
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
		}

		BeforeEach(func() {
			location := time.FixedZone("foo", 0)

			symlinkTargetFilePath := path.Join(baseImagePath, "symlink-target")
			Expect(ioutil.WriteFile(symlinkTargetFilePath, []byte("hello-world"), 0600)).To(Succeed())
			symlinkFilePath := path.Join(baseImagePath, "old-symlink")
			Expect(os.Symlink("./symlink-target", symlinkFilePath)).To(Succeed())

			symlinkModTime = time.Date(2014, 11, 5, 12, 8, 32, 0, location)
			setSymlinkModtime(symlinkFilePath, symlinkModTime)
		})

		It("preserves the modtime for symlinks", func() {
			Expect(tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})).To(Succeed())

			symlinkTargetFi, err := os.Stat(path.Join(targetPath, "symlink-target"))
			Expect(err).NotTo(HaveOccurred())

			symlinkFi, err := os.Lstat(path.Join(targetPath, "old-symlink"))
			Expect(err).NotTo(HaveOccurred())

			Expect(symlinkTargetFi.ModTime().Unix()).NotTo(Equal(symlinkFi.ModTime().Unix()))
			Expect(symlinkFi.ModTime().Unix()).To(Equal(symlinkModTime.Unix()))
		})
	})
})

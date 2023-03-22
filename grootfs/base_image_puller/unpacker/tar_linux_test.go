package unpacker_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Tar unpacker - Linux tests", func() {
	var (
		tarUnpacker *unpacker.TarUnpacker

		storeDir      string
		storeDirFile  *os.File
		logger        lager.Logger
		baseImagePath string
		stream        *os.File
		targetPath    string
		tarCommand    *exec.Cmd
		tarFilePath   string
	)

	BeforeEach(func() {
		var err error
		storeDir, err = ioutil.TempDir("", "store-")
		Expect(err).NotTo(HaveOccurred())

		targetPath = "."

		storeDirFile, err = os.Open(storeDir)
		Expect(err).NotTo(HaveOccurred())

		mappings := []groot.IDMappingSpec{
			{HostID: 5000, NamespaceID: 0, Size: 1},
			{HostID: 100000, NamespaceID: 1, Size: 65000},
		}
		tarUnpacker = unpacker.NewTarUnpacker(
			unpacker.NewOverlayWhiteoutHandler(storeDirFile),
			unpacker.NewIDTranslator(mappings, mappings),
		)

		baseImagePath, err = ioutil.TempDir("", "base-image-")
		Expect(err).NotTo(HaveOccurred())

		tarFilePath = filepath.Join(os.TempDir(), (fmt.Sprintf("unpack-test-%d.tar", GinkgoParallelProcess())))
		tarCommand = exec.Command("tar", "-C", baseImagePath, "-cf", tarFilePath, ".")

		logger = lagertest.NewTestLogger("test-store")
	})

	JustBeforeEach(func() {
		var err error

		Expect(tarCommand.Run()).To(Succeed())
		stream, err = os.OpenFile(tarFilePath, os.O_RDONLY, 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(stream.Close()).To(Succeed())
		Expect(storeDirFile.Close()).To(Succeed())
		Expect(os.RemoveAll(storeDir)).To(Succeed())
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
		Expect(os.RemoveAll(tarFilePath)).To(Succeed())
	})

	Describe("Devices files", func() {
		BeforeEach(func() {
			Expect(exec.Command("sudo", "mknod", path.Join(baseImagePath, "a_device"), "c", "1", "8").Run()).To(Succeed())
		})

		It("excludes them", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

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
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			symlinkTargetFi, err := os.Stat(path.Join(targetPath, "symlink-target"))
			Expect(err).NotTo(HaveOccurred())

			symlinkFi, err := os.Lstat(path.Join(targetPath, "old-symlink"))
			Expect(err).NotTo(HaveOccurred())

			Expect(symlinkTargetFi.ModTime().Unix()).NotTo(Equal(symlinkFi.ModTime().Unix()))
			Expect(symlinkFi.ModTime().Unix()).To(Equal(symlinkModTime.Unix()))
		})
	})

	Describe("file ownership", func() {
		BeforeEach(func() {
			filepath := path.Join(baseImagePath, "myfile")
			Expect(ioutil.WriteFile(filepath, []byte{}, 0755)).To(Succeed())
		})

		It("maps ownership", func() {
			_, err := tarUnpacker.Unpack(logger, base_image_puller.UnpackSpec{
				Stream:     stream,
				TargetPath: targetPath,
			})
			Expect(err).NotTo(HaveOccurred())

			stat, err := os.Stat(filepath.Join(targetPath, "myfile"))
			Expect(err).NotTo(HaveOccurred())
			stat_t := stat.Sys().(*syscall.Stat_t)
			Expect(stat_t.Uid).To(Equal(uint32(5000)))
			Expect(stat_t.Gid).To(Equal(uint32(5000)))
		})
	})
})

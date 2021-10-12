package unpacker_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/base_image_puller/unpacker"
	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

var _ = Describe("WhiteoutHandler", func() {
	var (
		storePath       string
		storeFile       *os.File
		filePath        string
		whiteoutPath    string
		whiteoutHandler unpacker.WhiteoutHandler
		deviceNumber    uint64
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "store-")
		Expect(err).NotTo(HaveOccurred())

		whiteoutDevicePath := filepath.Join(storePath, overlayxfs.WhiteoutDevice)
		deviceNumber = unix.Mkdev(123, 456)
		Expect(unix.Mknod(whiteoutDevicePath, unix.S_IFCHR, int(deviceNumber))).To(Succeed())

		storeFile, err = os.Open(storePath)
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(storePath, "layer-1"), 0755)).To(Succeed())
		filePath = filepath.Join(storePath, "layer-1", "thefile")
		Expect(ioutil.WriteFile(filePath, []byte{}, 0755)).To(Succeed())
		whiteoutPath = filepath.Join(storePath, "layer-1", ".wh.thefile")
		Expect(ioutil.WriteFile(whiteoutPath, []byte{}, 0755)).To(Succeed())

		whiteoutHandler = unpacker.NewOverlayWhiteoutHandler(storeFile)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	It("creates dev 0 character devices to simulate file deletions", func() {
		Expect(whiteoutHandler.RemoveWhiteout(whiteoutPath)).To(Succeed())
		stat, err := os.Stat(filePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(stat.Mode()).To(Equal(os.ModeCharDevice|os.ModeDevice), "Whiteout file is not a character device")
		stat_t := stat.Sys().(*syscall.Stat_t)
		Expect(stat_t.Rdev).To(Equal(deviceNumber), "Whiteout file has incorrect device number")
	})
})

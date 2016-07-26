package root_test

import (
	"io/ioutil"
	"os"
	"path"
	"syscall"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	var (
		imagePath string
		rootUID   int
		rootGID   int
	)

	BeforeEach(func() {
		rootUID = 0
		rootGID = 0

		var err error
		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		os.Chown(imagePath, rootUID, rootGID)

		Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0700)).To(Succeed())
		os.Chown(path.Join(imagePath, "foo"), rootUID, rootGID)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	It("keeps the ownership and permissions", func() {
		bundlePath := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "random-id")

		stat, err := os.Stat(path.Join(bundlePath, "rootfs", "foo"))
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(BeEquivalentTo(rootUID))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(BeEquivalentTo(rootGID))
	})
})

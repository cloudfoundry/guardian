package filesystems_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store/filesystems"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Common", func() {
	var logger *lagertest.TestLogger

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("logger")
	})

	Describe("CalculatePathSize", func() {
		var path string

		BeforeEach(func() {
			var err error
			path, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Mkdir(filepath.Join(path, "directory"), 0755)).To(Succeed())

			writeFile(filepath.Join(path, "directory", "file-1"), 10048)
			writeFile(filepath.Join(path, "file-2"), 5048)

			Expect(os.Symlink(filepath.Join(path, "directory", "file-1"), filepath.Join(path, "link-1"))).To(Succeed())
			Expect(os.Link(filepath.Join(path, "file-2"), filepath.Join(path, "link-2"))).To(Succeed())

		})

		AfterEach(func() {
			Expect(os.RemoveAll(path)).To(Succeed())
		})

		It("returns the correct path size", func() {
			size, err := filesystems.CalculatePathSize(logger, path)
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(BeNumerically("~", 15096, 100))
		})
	})

})

func writeFile(path string, size int64) {
	cmd := exec.Command("dd", "if=/dev/zero", "of="+path, fmt.Sprintf("bs=%d", size), "count=1")
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
}

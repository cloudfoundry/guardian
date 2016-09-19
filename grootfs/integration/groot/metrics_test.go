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

var _ = Describe("Metrics", func() {
	var imagePath string

	BeforeEach(func() {
		var err error
		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	It("returns the metrics for given bundle", func() {
		cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(imagePath, "fatfile")), "bs=1048576", "count=5")
		sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		bundle := integration.CreateBundle(GrootFSBin, StorePath, imagePath, "random-id", 0)

		cmd = exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", filepath.Join(bundle.RootFSPath(), "hello")), "bs=1048576", "count=4")
		sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		cmd = exec.Command(GrootFSBin, "--store", StorePath, "metrics", "random-id")
		sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		Eventually(sess.Out).Should(gbytes.Say(`{"disk_usage":{"total_bytes_used":9453568,"exclusive_bytes_used":4210688}}`))
	})

	Context("when the bundle id doesn't exist", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "metrics", "invalid-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("No such file or directory"))
		})
	})

	Context("when the bundle id is not provided", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "metrics")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Out).Should(gbytes.Say("invalid arguments"))
		})
	})
})

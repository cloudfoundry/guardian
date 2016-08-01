package groot_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"code.cloudfoundry.org/grootfs/graph"
	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Delete", func() {
	var (
		bundlePath string
	)

	BeforeEach(func() {
		bundlePath = path.Join(GraphPath, graph.BUNDLES_DIR_NAME, "random-id")
		Expect(os.MkdirAll(bundlePath, 0755)).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(bundlePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())
	})

	It("deletes an existing bundle", func() {
		result := integration.DeleteBundle(GrootFSBin, GraphPath, "random-id")
		Expect(result).To(Equal("Bundle random-id deleted\n"))
		Expect(path.Join(bundlePath)).NotTo(BeAnExistingFile())
	})

	Context("when the bundle ID doesn't exist", func() {
		It("returns an error", func() {
			cmd := exec.Command(GrootFSBin, "--graph", GraphPath, "delete", "non-existing-id")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Err).Should(gbytes.Say("bundle path not found"))
		})
	})

	Context("when the id is not provided", func() {
		It("fails", func() {
			cmd := exec.Command(GrootFSBin, "--graph", GraphPath, "delete")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))
			Eventually(sess.Err).Should(gbytes.Say("id was not specified"))
		})
	})
})

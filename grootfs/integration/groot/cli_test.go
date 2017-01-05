package groot_test

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("CLI", func() {
	Context("when no action is provided", func() {
		It("does not configure the store", func() {
			cmd := exec.Command(GrootFSBin)
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			Eventually(sess.Out).ShouldNot(gbytes.Say("making directory"))

			fileinfo, err := os.Stat(store.DEFAULT_STORE_PATH)
			Expect(fileinfo).To(BeNil())
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})
})

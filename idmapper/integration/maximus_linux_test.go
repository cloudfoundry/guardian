package integration_test

import (
	"os/exec"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Maximus", func() {
	It("returns the maximus uid", func() {
		sess, err := gexec.Start(exec.Command(MaximusBin), GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		Eventually(sess.Out).Should(gbytes.Say(strconv.FormatUint(uint64(MaximusID), 10)))
	})
})

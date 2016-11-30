package testhelpers

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func SuidDrax(draxBinPath string) {
	sess, err := gexec.Start(
		exec.Command("sudo", "chown", "root:root", draxBinPath),
		GinkgoWriter, GinkgoWriter,
	)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	sess, err = gexec.Start(
		exec.Command("sudo", "chmod", "u+s", draxBinPath),
		GinkgoWriter, GinkgoWriter,
	)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, "5s").Should(gexec.Exit(0))
}

func UnsuidDrax(draxBinPath string) {
	sess, err := gexec.Start(
		exec.Command("sudo", "chmod", "u-s", draxBinPath),
		GinkgoWriter, GinkgoWriter,
	)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
}

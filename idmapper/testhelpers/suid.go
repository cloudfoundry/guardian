package testhelpers

import (
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func Suid(binPath string) {
	sess, err := gexec.Start(
		exec.Command("sudo", "chown", "root:root", binPath),
		GinkgoWriter, GinkgoWriter,
	)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	sess, err = gexec.Start(
		exec.Command("sudo", "chmod", "u+s", binPath),
		GinkgoWriter, GinkgoWriter,
	)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
}

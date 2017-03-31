package testhelpers

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func CopyFile(source, destination string) {
	cmd := exec.Command("cp", "-r", source, destination)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess, 15*time.Second).Should(gexec.Exit(0))
}

package stopper_test

import (
	"os/exec"
	"syscall"

	"code.cloudfoundry.org/guardian/rundmc/stopper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Killer", func() {
	It("kills the given processes", func() {
		cmd1 := exec.Command("powershell.exe", "-Command", "while ($true) { Write-Host 'trapping'; Start-Sleep 1 }")
		sess1, err := gexec.Start(cmd1, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		cmd2 := exec.Command("powershell.exe", "-Command", "while ($true) { Write-Host 'trapping'; Start-Sleep 1 }")
		sess2, err := gexec.Start(cmd2, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())

		Eventually(sess1).Should(gbytes.Say("trapping"))
		Eventually(sess2).Should(gbytes.Say("trapping"))

		stopper.DefaultKiller{}.Kill(syscall.SIGKILL, cmd1.Process.Pid, cmd2.Process.Pid)

		Eventually(sess1, "5s").Should(gexec.Exit(1))
		Eventually(sess2, "5s").Should(gexec.Exit(1))
	})
})

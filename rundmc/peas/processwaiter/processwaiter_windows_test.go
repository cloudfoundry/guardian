package processwaiter_test

import (
	"os"
	"os/exec"

	"code.cloudfoundry.org/guardian/rundmc/peas/processwaiter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Process Waiter", func() {
	var waiter processwaiter.ProcessWaiter

	BeforeEach(func() {
		waiter = processwaiter.WaitOnProcess
	})

	It("waits for the process to finish", func() {
		cmd := exec.Command("cmd.exe", "set", "/p")
		processIn, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())

		Expect(cmd.Start()).To(Succeed())
		_, err = os.FindProcess(cmd.Process.Pid)
		Expect(err).NotTo(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			Expect(waiter.Wait(cmd.Process.Pid)).To(Succeed())
			_, err := os.FindProcess(cmd.Process.Pid)
			Expect(err).To(HaveOccurred())
		}()

		processIn.Close()
	})
})

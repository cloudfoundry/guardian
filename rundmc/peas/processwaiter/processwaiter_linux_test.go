package processwaiter_test

import (
	"fmt"
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
		cmd := exec.Command("/bin/sh", "read")
		processIn, err := cmd.StdinPipe()
		Expect(err).NotTo(HaveOccurred())

		Expect(cmd.Start()).To(Succeed())

		pid := cmd.Process.Pid
		Expect(fmt.Sprintf("/proc/%d", pid)).To(BeADirectory())

		go func() {
			defer GinkgoRecover()
			Expect(waiter.Wait(pid)).To(Succeed())
			Expect(fmt.Sprintf("/proc/%d", pid)).NotTo(BeADirectory())
		}()

		processIn.Close()
	})
})

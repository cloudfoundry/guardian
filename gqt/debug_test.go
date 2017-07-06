package gqt_test

import (
	"fmt"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Debug Endpoint", func() {
	var (
		client *runner.RunningGarden
	)

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	It("does not listen for debug", func() {
		out, err := exec.Command("/bin/sh", "-c", fmt.Sprintf("netstat -anp | grep %d", client.Pid)).CombinedOutput()
		Expect(err).NotTo(HaveOccurred())

		output := strings.TrimSpace(string(out))
		Expect(output).NotTo(ContainSubstring("tcp"))
	})

	Context("when garden is started with debug address", func() {
		BeforeEach(func() {
			config.DebugIP = "127.0.0.1"
			config.DebugPort = intptr(9876)
		})

		It("listens on the specified address only", func() {
			out, err := exec.Command("/bin/sh", "-c", "netstat -na | grep ':9876'").CombinedOutput()
			Expect(err).NotTo(HaveOccurred())

			output := strings.TrimSpace(string(out))
			Expect(output).To(ContainSubstring("127.0.0.1:9876"))
			Expect(output).To(ContainSubstring("tcp"))
			Expect(len(strings.Split(output, "\n"))).To(Equal(1))
		})
	})
})

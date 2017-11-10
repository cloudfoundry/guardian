package gqt_test

import (
	"fmt"
	"syscall"

	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("gdn server", func() {
	var (
		server *runner.RunningGarden
	)

	JustBeforeEach(func() {
		config.Tag = fmt.Sprintf("%d", GinkgoParallelNode())
		config.User = &syscall.Credential{Uid: 0, Gid: 0}
		server = runner.Start(config)
	})

	AfterEach(func() {
		Expect(server.DestroyAndStop()).To(Succeed())
	})

	Context("when we start the server on an IP and port", func() {
		BeforeEach(func() {
			config.BindIP = "127.0.0.1"
			config.BindPort = intptr(54321)
			config.BindSocket = ""
		})

		Context("when we start the server again with the same IP and port", func() {
			It("crashes", func() {
				client := runner.Start(config)
				Eventually(client).Should(gbytes.Say("listen tcp 127.0.0.1:54321: bind: address already in use"))
			})
		})
	})
})

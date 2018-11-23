package gqt_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Config File", func() {
	Describe("starting gdn server with the --config flag", func() {
		var (
			configFilePath string
			port           int
			client         *runner.RunningGarden
		)

		BeforeEach(func() {
			config.DebugIP = ""
			config.DebugPort = nil

			configFile := tempFile("", "gqt-config-file-tests")
			configFilePath = configFile.Name()

			port = 9080 + GinkgoParallelNode()
			fmt.Fprintf(configFile, `[server]
debug-bind-ip = 127.0.0.1
debug-bind-port = %d
`, port)
			Expect(configFile.Close()).To(Succeed())
			config.ConfigFilePath = configFilePath
		})

		JustBeforeEach(func() {
			client = runner.Start(config)
		})

		AfterEach(func() {
			if !config.StartupExpectedToFail {
				Expect(client.DestroyAndStop()).To(Succeed())
			}
			Expect(os.RemoveAll(configFilePath)).To(Succeed())
		})

		It("starts the server with config values from the config file", func() {
			conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			Expect(err).NotTo(HaveOccurred())
			Expect(conn.Close()).To(Succeed())
		})

		Context("and also passing flags on the command line that override values in the config file", func() {
			BeforeEach(func() {
				config.DebugPort = intptr(7080 + GinkgoParallelNode())
			})

			It("starts the server with config values from the command line taking precedence over the config file", func() {
				conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", *config.DebugPort))
				Expect(err).NotTo(HaveOccurred())
				Expect(conn.Close()).To(Succeed())
			})
		})

		Context("when the provided config file is not a valid ini file", func() {
			BeforeEach(func() {
				config.StartupExpectedToFail = true
				Expect(ioutil.WriteFile(configFilePath, []byte("invalid-ini-file"), 0)).To(Succeed())
			})

			It("fails to start", func() {
				Eventually(client).Should(gexec.Exit(1))
			})
		})

		Context("when the provided config file path does not exist", func() {
			BeforeEach(func() {
				config.StartupExpectedToFail = true
				Expect(os.Remove(configFilePath)).To(Succeed())
			})

			It("fails to start", func() {
				Eventually(client).Should(gexec.Exit(1))
			})
		})
	})
})

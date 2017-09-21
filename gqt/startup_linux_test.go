package gqt_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/guardian/gqt/runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Startup", func() {
	Describe("starting up with socket2me", func() {
		var (
			client  *runner.RunningGarden
			tempDir string
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "gqt-startup-test")
			Expect(err).NotTo(HaveOccurred())

			tmpDir := filepath.Join(
				os.TempDir(),
				fmt.Sprintf("test-garden-%s", config.Tag),
			)
			setupArgs := []string{"setup",
				"--tag", config.Tag,
				"--rootless-uid", idToStr(unprivilegedUID),
				"--rootless-gid", idToStr(unprivilegedGID)}

			cmd := exec.Command(binaries.Gdn, setupArgs...)
			cmd.Env = append(
				[]string{
					fmt.Sprintf("TMPDIR=%s", tmpDir),
					fmt.Sprintf("TEMP=%s", tmpDir),
					fmt.Sprintf("TMP=%s", tmpDir),
				},
				os.Environ()...,
			)

			setupProcess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(setupProcess).Should(gexec.Exit(0))

			config.BindSocket = ""
			config.Socket2meSocketPath = filepath.Join(tempDir, "socket.sock")

			config.SkipSetup = boolptr(true)
			config.User = &syscall.Credential{Uid: 5000, Gid: 5000}
			config.GraphDir = "" // We don't need to create containers in this test
			config.ImagePluginBin = binaries.NoopPlugin
			config.NetworkPluginBin = binaries.NoopPlugin
			client = runner.Start(config)
		})

		AfterEach(func() {
			Expect(client.DestroyAndStop()).To(Succeed())
			Expect(os.RemoveAll(tempDir)).To(Succeed())
		})

		It("starts up", func() {
			Consistently(client.Ping).Should(Succeed())
		})
	})
})

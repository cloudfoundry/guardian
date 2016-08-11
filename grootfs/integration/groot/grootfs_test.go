package groot_test

import (
	"io/ioutil"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("grootfs global flags", func() {
	Describe("logs", func() {
		It("forwards human logs to stdout", func() {
			cmd := exec.Command(GrootFSBin, "--store", StorePath, "create", "--image", "my-image")
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(1))

			Expect(err).NotTo(HaveOccurred())
			Eventually(sess.Out).Should(gbytes.Say("id was not specified"))
		})

		Context("when setting --verbose", func() {
			It("forwards non-human logs to stderr", func() {
				cmd := exec.Command(GrootFSBin, "--log-level", "error", "--store", StorePath, "create", "--image", "my-image")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))

				Expect(err).NotTo(HaveOccurred())
				Eventually(sess.Err).Should(gbytes.Say(`"data":{"error":"id was not specified"}`))
			})
		})

		Context("when providing a --log-file", func() {
			var (
				logFile *os.File
			)

			BeforeEach(func() {
				logPath, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				logFile, err = ioutil.TempFile(logPath, "mylog")
				Expect(err).NotTo(HaveOccurred())
			})

			It("forwards logs to the given file", func() {
				cmd := exec.Command(GrootFSBin, "--log-level", "debug", "--log-file", logFile.Name(), "--store", StorePath, "create", "--image", "my-image")
				sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
				Expect(err).NotTo(HaveOccurred())
				Eventually(sess).Should(gexec.Exit(1))

				log, err := ioutil.ReadFile(logFile.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(log).To(ContainSubstring("id was not specified"))
			})
		})
	})
})

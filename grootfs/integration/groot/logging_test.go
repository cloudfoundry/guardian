package groot_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Logging", func() {
	It("forwards human ouput to stdout", func() {
		buffer := gbytes.NewBuffer()

		_, err := Runner.
			WithStdout(buffer).
			Create(groot.CreateSpec{
				ID:        "my-image",
				BaseImage: "/non/existent/rootfs",
			})
		Expect(err).To(HaveOccurred())

		Eventually(buffer).Should(gbytes.Say("no such file or directory"))
	})

	It("re-logs the nested unpack commands logs", func() {
		imgPath, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(
			filepath.Join(imgPath, "unreadable-file"), []byte("foo bar"), 0644,
		)).To(Succeed())

		logBuffer := gbytes.NewBuffer()
		_, err = Runner.WithStderr(logBuffer).Create(groot.CreateSpec{
			ID:        "random-id",
			BaseImage: imgPath,
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(logBuffer).To(gbytes.Say("namespaced-unpacking.unpack"))
	})

	Describe("--log-level and --log-file flags", func() {
		Context("when the --log-file is not set", func() {
			Context("and --log-level is set", func() {
				It("writes logs to stderr", func() {
					buffer := gbytes.NewBuffer()

					_, err := Runner.
						WithStderr(buffer).
						WithLogLevel(lager.DEBUG).
						Create(groot.CreateSpec{
							ID:        "my-image",
							BaseImage: "/non/existent/rootfs",
						})
					Expect(err).To(HaveOccurred())

					Expect(buffer).To(gbytes.Say(`"error":".*no such file or directory"`))
				})
			})
		})

		Context("when the --log-file is set", func() {
			var (
				logFilePath string
				logReader   io.ReadCloser
				logWriter   io.WriteCloser
			)

			BeforeEach(func() {
				r, w, err := os.Pipe()
				Expect(err).NotTo(HaveOccurred())

				logFilePath = fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), w.Fd())
				logReader = r
				logWriter = w
			})

			AfterEach(func() {
				Expect(logReader.Close()).To(Succeed())
			})

			Context("and --log-level is set", func() {
				It("forwards logs to the given file", func() {
					_, err := Runner.
						WithLogFile(logFilePath).
						WithLogLevel(lager.DEBUG).
						Create(groot.CreateSpec{
							ID:        "my-image",
							BaseImage: "/non/existent/rootfs",
						})
					Expect(err).To(HaveOccurred())

					Expect(logWriter.Close()).To(Succeed())
					allTheLogs, err := ioutil.ReadAll(logReader)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(allTheLogs)).To(ContainSubstring("\"log_level\":0"))
				})
			})
		})
	})
})

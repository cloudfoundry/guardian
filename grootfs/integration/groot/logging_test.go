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
})

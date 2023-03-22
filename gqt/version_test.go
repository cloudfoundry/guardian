package gqt_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"os/exec"
)

var _ = Describe("gdn -v", func() {
	var (
		pathToGdn      string
		gdnCompileArgs []string
		version        []byte
	)

	BeforeEach(func() {
		gdnCompileArgs = []string{}
	})

	JustBeforeEach(func() {
		var err error

		pathToGdn = CompileGdn(gdnCompileArgs...)

		versionCmd := exec.Command(pathToGdn, "-v")
		version, err = versionCmd.Output()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(filepath.Dir(pathToGdn))
	})

	It("reports 'dev' as the version by default", func() {
		Expect(version).To(ContainSubstring("dev"))
	})

	Context("when compiled with -ldflags '-X main.version'", func() {
		BeforeEach(func() {
			gdnCompileArgs = []string{"-ldflags", "-X main.version=test.version.x"}
		})

		It("prints the version specified in main.version", func() {
			Expect(version).To(ContainSubstring("test.version.x"))
		})
	})
})

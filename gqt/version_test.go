package gqt_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"

	"github.com/onsi/gomega/gexec"
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
		gexec.CleanupBuildArtifacts()
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

package gqt_setup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.cloudfoundry.org/guardian/gqt/runner"
	"github.com/BurntSushi/toml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	binaries runner.Binaries
)

func TestSetupGqt(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Setup Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	binaries := runner.Binaries{
		Gdn: compileGdn(),
	}

	// chmod all the artifacts
	Expect(os.Chmod(filepath.Join(binaries.Gdn, "..", ".."), 0755)).To(Succeed())
	filepath.Walk(filepath.Join(binaries.Gdn, "..", ".."), func(path string, info os.FileInfo, err error) error {
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(path, 0755)).To(Succeed())
		return nil
	})

	return jsonMarshal(binaries)
}, func(data []byte) {
	bins := new(runner.Binaries)
	jsonUnmarshal(data, bins)
	binaries = *bins
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

func compileGdn(additionalCompileArgs ...string) string {
	defaultCompileArgs := []string{"-tags", "daemon"}
	compileArgs := append(defaultCompileArgs, additionalCompileArgs...)

	return goCompile("code.cloudfoundry.org/guardian/cmd/gdn", compileArgs...)
}

func goCompile(mainPackagePath string, buildArgs ...string) string {
	if os.Getenv("RACE_DETECTION") != "" {
		buildArgs = append(buildArgs, "-race")
	}
	buildArgs = append(buildArgs, "-mod=vendor")
	bin, err := gexec.Build(mainPackagePath, buildArgs...)
	Expect(err).NotTo(HaveOccurred())
	return bin
}

func jsonMarshal(v interface{}) []byte {
	buf := bytes.NewBuffer([]byte{})
	Expect(toml.NewEncoder(buf).Encode(v)).To(Succeed())
	return buf.Bytes()
}

func jsonUnmarshal(data []byte, v interface{}) {
	Expect(toml.Unmarshal(data, v)).To(Succeed())
}

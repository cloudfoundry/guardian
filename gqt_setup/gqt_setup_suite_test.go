package gqt_setup_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.cloudfoundry.org/guardian/gqt/helpers"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	// the unprivileged user is baked into the cfgarden/garden-ci-ubuntu image
	unprivilegedUID = uint32(5000)
	unprivilegedGID = uint32(5000)

	binaries runner.Binaries
)

func TestSetupGqt(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Setup Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	binaries := runner.Binaries{
		Gdn: helpers.CompileGdn(),
	}

	// chmod all the artifacts
	Expect(os.Chmod(filepath.Join(binaries.Gdn, "..", ".."), 0755)).To(Succeed())
	filepath.Walk(filepath.Join(binaries.Gdn, "..", ".."), func(path string, info os.FileInfo, err error) error {
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(path, 0755)).To(Succeed())
		return nil
	})

	return helpers.JsonMarshal(binaries)
}, func(data []byte) {
	bins := new(runner.Binaries)
	helpers.JsonUnmarshal(data, bins)
	binaries = *bins
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

// E.g. nodeToString(1) = a, nodeToString(2) = b, etc ...
func nodeToString(ginkgoNode int) string {
	r := 'a' + ginkgoNode - 1
	Expect(r).To(BeNumerically(">=", 'a'))
	Expect(r).To(BeNumerically("<=", 'z'))
	return string(r)
}

func idToStr(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}

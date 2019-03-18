package gqt_cleanup_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"code.cloudfoundry.org/guardian/gqt/helpers"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	binaries          runner.Binaries
	config            runner.GdnRunnerConfig
	defaultTestRootFS string
)

func TestSetupGqt(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(5 * time.Second)
	RunSpecs(t, "GQT Cleanup Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	binaries := helpers.GetGardenBinaries()

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
	defaultTestRootFS = os.Getenv("GARDEN_TEST_ROOTFS")
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = BeforeEach(func() {
	if defaultTestRootFS == "" {
		Skip("No Garden RootFS")
	}
	config = helpers.DefaultConfig(defaultTestRootFS, binaries)
})

var _ = AfterEach(func() {
	// Windows worker is not containerised and therefore the test needs to take care to delete the temporary folder
	if runtime.GOOS == "windows" {
		Expect(os.RemoveAll(config.TmpDir)).To(Succeed())
	}
})

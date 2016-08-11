package groot_test

import (
	"fmt"
	"os"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	GrootFSBin string

	StorePath string
)

func TestGroot(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		grootFSBin, err := gexec.Build("code.cloudfoundry.org/grootfs")
		Expect(err).NotTo(HaveOccurred())

		return []byte(grootFSBin)
	}, func(data []byte) {
		GrootFSBin = string(data)
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		StorePath = path.Join(os.TempDir(), fmt.Sprintf("test-store-%d", GinkgoParallelNode()))
		Expect(os.Mkdir(StorePath, 0700)).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(StorePath)).To(Succeed())
	})

	RunSpecs(t, "GrootFS Integration Suite - Running as groot")
}

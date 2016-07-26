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

	GraphPath string
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
		GraphPath = path.Join(os.TempDir(), fmt.Sprintf("test-graph-%d", GinkgoParallelNode()))
		Expect(os.Mkdir(GraphPath, 0700)).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(GraphPath)).To(Succeed())
	})

	RunSpecs(t, "GrootFS Integration Suite - Running as groot")
}

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

	testIdx   int
	StorePath string
)

const btrfsMountPath = "/mnt/btrfs"

func TestGroot(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		grootFSBin, err := gexec.Build("code.cloudfoundry.org/grootfs")
		Expect(err).NotTo(HaveOccurred())

		return []byte(grootFSBin)
	}, func(data []byte) {
		GrootFSBin = string(data)
		testIdx = 0
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		StorePath = path.Join(
			btrfsMountPath,
			fmt.Sprintf("test-store-%d-%d", GinkgoParallelNode(), testIdx),
		)
		Expect(os.Mkdir(StorePath, 0700)).NotTo(HaveOccurred())
		testIdx += 1
	})

	RunSpecs(t, "GrootFS Integration Suite - Running as groot")
}

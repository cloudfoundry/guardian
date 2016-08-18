package root_test

import (
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	GrootFSBin string

	GrootUID uint32
	GrootGID uint32

	testIdx   int
	StorePath string
)

const btrfsMountPath = "/mnt/btrfs"

func TestRoot(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		grootFSBin, err := gexec.Build("code.cloudfoundry.org/grootfs")
		Expect(err).NotTo(HaveOccurred())

		fixPermission(path.Dir(grootFSBin))

		return []byte(grootFSBin)
	}, func(data []byte) {
		GrootUID = integration.FindUID("groot")
		GrootGID = integration.FindGID("groot")
		GrootFSBin = string(data)
		testIdx = 0
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		if os.Getuid() != 0 {
			Skip("This suite is only running as root")
		}

		StorePath = path.Join(
			btrfsMountPath,
			fmt.Sprintf("test-store-%d-%d", GinkgoParallelNode(), testIdx),
		)
		Expect(os.Mkdir(StorePath, 0700)).NotTo(HaveOccurred())
		testIdx += 1

		Expect(os.Chown(StorePath, int(GrootUID), int(GrootGID))).To(Succeed())
	})

	RunSpecs(t, "GrootFS Integration Suite - Running as root")
}

func fixPermission(dirPath string) {
	fi, err := os.Stat(dirPath)
	Expect(err).NotTo(HaveOccurred())
	if !fi.IsDir() {
		return
	}

	// does other have the execute permission?
	if mode := fi.Mode(); mode&01 == 0 {
		Expect(os.Chmod(dirPath, 0755)).To(Succeed())
	}

	if dirPath == "/" {
		return
	}
	fixPermission(path.Dir(dirPath))
}

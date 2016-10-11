package root_test

import (
	"fmt"
	"os"
	"path"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	GrootFSBin string
	DraxBin    string

	GrootUID uint32
	GrootGID uint32

	storeName string
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
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		if os.Getuid() != 0 {
			Skip("This suite is only running as root")
		}

		storeName = fmt.Sprintf("test-store-%d", GinkgoParallelNode())
		StorePath = path.Join(btrfsMountPath, storeName)
		Expect(os.Mkdir(StorePath, 0700)).NotTo(HaveOccurred())

		Expect(os.Chown(StorePath, int(GrootUID), int(GrootGID))).To(Succeed())

		var err error
		DraxBin, err = gexec.Build("code.cloudfoundry.org/grootfs/store/volume_driver/drax")
		Expect(err).NotTo(HaveOccurred())
		testhelpers.SuidDrax(DraxBin)
	})

	AfterEach(func() {
		testhelpers.CleanUpSubvolumes(btrfsMountPath, storeName)
		Expect(os.RemoveAll(StorePath)).To(Succeed())
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

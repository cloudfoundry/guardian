package groot_test

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	GrootFSBin string
	DraxBin    string
	StorePath  string
	Runner     runner.Runner
	storeName  string

	CurrentUserID string
)

const btrfsMountPath = "/mnt/btrfs"

func TestGroot(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		grootFSBin, err := gexec.Build("code.cloudfoundry.org/grootfs")
		Expect(err).NotTo(HaveOccurred())

		return []byte(grootFSBin)
	}, func(data []byte) {
		userID := os.Getuid()
		CurrentUserID = strconv.Itoa(userID)
		GrootFSBin = string(data)
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		if os.Getuid() == 0 {
			Skip("This suite is only running as groot")
		}

		storeName = fmt.Sprintf("test-store-%d", GinkgoParallelNode())
		StorePath = path.Join(btrfsMountPath, storeName)
		Expect(os.Mkdir(StorePath, 0755)).NotTo(HaveOccurred())

		var err error
		DraxBin, err = gexec.Build("code.cloudfoundry.org/grootfs/store/volume_driver/drax")
		Expect(err).NotTo(HaveOccurred())
		testhelpers.SuidDrax(DraxBin)

		r := runner.Runner{
			GrootFSBin: GrootFSBin,
			StorePath:  StorePath,
			DraxBin:    DraxBin,
		}
		Runner = r.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter)
	})

	AfterEach(func() {
		testhelpers.CleanUpSubvolumes(btrfsMountPath, storeName)
		Expect(os.RemoveAll(StorePath)).To(Succeed())
	})

	RunSpecs(t, "GrootFS Integration Suite - Running as groot")
}

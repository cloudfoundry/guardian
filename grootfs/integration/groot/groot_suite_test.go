package groot_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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

	CurrentUserID    string
	RegistryUsername string
	RegistryPassword string
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

		RegistryUsername = os.Getenv("REGISTRY_USERNAME")
		RegistryPassword = os.Getenv("REGISTRY_PASSWORD")

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

func writeMegabytes(outputPath string, mb int) error {
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", outputPath), "bs=1048576", fmt.Sprintf("count=%d", mb))
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}
	Eventually(sess).Should(gexec.Exit())
	if sess.ExitCode() > 0 {
		return errors.New(string(sess.Err.Contents()))
	}
	return nil
}

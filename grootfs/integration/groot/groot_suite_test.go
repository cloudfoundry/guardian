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
	Driver     string

	CurrentUserID    string
	CurrentUserIDInt int
	RegistryUsername string
	RegistryPassword string
)

const btrfsMountPath = "/mnt/btrfs"
const xfsMountPath = "/mnt/xfs"

func TestOverlayXfs(t *testing.T) {
	grootTests(t, "overlay-xfs", xfsMountPath)
}

func TestBtrfs(t *testing.T) {
	grootTests(t, "btrfs", btrfsMountPath)
}

func grootTests(t *testing.T, driver string, mountPath string) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		grootFSBin, err := gexec.Build("code.cloudfoundry.org/grootfs")
		Expect(err).NotTo(HaveOccurred())

		return []byte(grootFSBin)
	}, func(data []byte) {
		CurrentUserIDInt = os.Getuid()
		CurrentUserID = strconv.Itoa(CurrentUserIDInt)
		GrootFSBin = string(data)
		Driver = driver
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		storeName = fmt.Sprintf("test-store-%d", GinkgoParallelNode())
		StorePath = path.Join(mountPath, storeName)
		Expect(os.Mkdir(StorePath, 0755)).NotTo(HaveOccurred())

		var err error
		DraxBin, err = gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax")
		Expect(err).NotTo(HaveOccurred())
		testhelpers.SuidDrax(DraxBin)

		RegistryUsername = os.Getenv("REGISTRY_USERNAME")
		RegistryPassword = os.Getenv("REGISTRY_PASSWORD")

		r := runner.Runner{
			GrootFSBin: GrootFSBin,
			StorePath:  StorePath,
			DraxBin:    DraxBin,
			Driver:     driver,
		}
		Runner = r.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter)
	})

	AfterEach(func() {
		if driver == "btrfs" {
			testhelpers.CleanUpBtrfsSubvolumes(mountPath, storeName)
		}

		if driver == "overlay-xfs" {
			testhelpers.CleanUpOverlayMounts(mountPath, storeName)
		}
		Expect(os.RemoveAll(StorePath)).To(Succeed())
	})

	RunSpecs(t, fmt.Sprintf("%s: GrootFS Integration Suite - Running as groot", driver))
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

package integration_test

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path"
	"strconv"
	"time"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

var (
	GrootFSBin    string
	DraxBin       string
	TardisBin     string
	Driver        string
	Runner        runner.Runner
	StorePath     string
	NamespacerBin string
	mountPath     string

	GrootUser        *user.User
	GrootUID         int
	GrootGID         int
	RegistryUsername string
	RegistryPassword string
	GrootfsTestUid   int
	GrootfsTestGid   int
)

const btrfsMountPath = "/mnt/btrfs-%d"
const xfsMountPath = "/mnt/xfs-%d"

func TestGroot(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		grootFSBin, err := gexec.Build("code.cloudfoundry.org/grootfs")
		Expect(err).NotTo(HaveOccurred())
		grootFSBin = integration.MakeBinaryAccessibleToEveryone(grootFSBin)

		return []byte(grootFSBin)
	}, func(data []byte) {
		var err error
		GrootUser, err = user.Lookup("groot")
		Expect(err).NotTo(HaveOccurred())

		tmpNamespacerBin, err := gexec.Build("code.cloudfoundry.org/grootfs/integration/namespacer")
		Expect(err).NotTo(HaveOccurred())

		rand.Seed(time.Now().UnixNano())
		NamespacerBin = fmt.Sprintf("/tmp/namespacer-%d", rand.Int())
		cpNamespacerBin := exec.Command("cp", tmpNamespacerBin, NamespacerBin)
		sess, err := gexec.Start(cpNamespacerBin, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess).Should(gexec.Exit(0))

		GrootUID, err = strconv.Atoi(GrootUser.Uid)
		Expect(err).NotTo(HaveOccurred())

		GrootGID, err = strconv.Atoi(GrootUser.Gid)
		Expect(err).NotTo(HaveOccurred())

		GrootFSBin = string(data)
		Driver = os.Getenv("VOLUME_DRIVER")

		GrootfsTestUid, _ = strconv.Atoi(os.Getenv("GROOTFS_TEST_UID"))
		GrootfsTestGid, _ = strconv.Atoi(os.Getenv("GROOTFS_TEST_GID"))

		fmt.Fprintf(os.Stderr, "============> RUNNING %s TESTS (%d:%d) <=============", Driver, GrootfsTestUid, GrootfsTestGid)
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		rand.Seed(time.Now().UnixNano() + int64(GinkgoParallelNode()*1000))

		if Driver == "overlay-xfs" {
			mountPath = fmt.Sprintf(xfsMountPath, GinkgoParallelNode())
		} else {
			Driver = "btrfs"
			mountPath = fmt.Sprintf(btrfsMountPath, GinkgoParallelNode())
		}
		StorePath = path.Join(mountPath, "store")

		var err error
		DraxBin, err = gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/btrfs/drax")
		Expect(err).NotTo(HaveOccurred())
		DraxBin = integration.MakeBinaryAccessibleToEveryone(DraxBin)
		testhelpers.SuidBinary(DraxBin)

		TardisBin, err = gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis")
		Expect(err).NotTo(HaveOccurred())
		TardisBin = integration.MakeBinaryAccessibleToEveryone(TardisBin)
		testhelpers.SuidBinary(TardisBin)

		RegistryUsername = os.Getenv("REGISTRY_USERNAME")
		RegistryPassword = os.Getenv("REGISTRY_PASSWORD")

		Runner = runner.Runner{
			GrootFSBin: GrootFSBin,
			StorePath:  StorePath,
			DraxBin:    DraxBin,
			TardisBin:  TardisBin,
			Driver:     Driver,
		}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).RunningAsUser(GrootfsTestUid, GrootfsTestGid)
	})

	AfterEach(func() {
		if Driver == "overlay-xfs" {
			testhelpers.CleanUpOverlayMounts(StorePath)
		} else {
			testhelpers.CleanUpBtrfsSubvolumes(mountPath)
		}

		Expect(os.RemoveAll(StorePath)).To(Succeed())
	})

	RunSpecs(t, "Integration Suite")
}

func writeMegabytes(outputPath string, mb int) error {
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", outputPath), "bs=1048576", fmt.Sprintf("count=%d", mb))
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	if err != nil {
		return err
	}
	Eventually(sess, 10*time.Second).Should(gexec.Exit())
	if sess.ExitCode() > 0 {
		return errors.New(string(sess.Err.Contents()))
	}
	return nil
}

func mountByDefault() bool {
	return GrootfsTestUid == 0 || isBtrfs()
}

func isBtrfs() bool {
	return Driver == "btrfs"
}

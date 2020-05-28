package integration_test

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var (
	GrootFSBin    string
	Runner        runner.Runner
	StorePath     string
	NamespacerBin string

	GrootUsername    string
	GrootUID         int
	GrootGID         int
	RegistryUsername string
	RegistryPassword string
	GrootfsTestUid   int
	GrootfsTestGid   int
	TardisBin        string
)

func TestGroot(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		testhelpers.EnableRootIDMapRange()

		grootFSBin, err := gexec.Build("code.cloudfoundry.org/grootfs", "-mod=vendor")
		Expect(err).NotTo(HaveOccurred())
		grootFSBin = integration.MakeBinaryAccessibleToEveryone(grootFSBin)

		tardisBin, err := gexec.Build("code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/tardis", "-mod=vendor")
		Expect(err).NotTo(HaveOccurred())
		tardisBin = integration.MakeBinaryAccessibleToEveryone(tardisBin)
		testhelpers.SuidBinary(tardisBin)

		namespacerBin, err := gexec.Build("code.cloudfoundry.org/grootfs/integration/namespacer", "-mod=vendor")
		Expect(err).NotTo(HaveOccurred())

		return []byte(grootFSBin + ":" + tardisBin + ":" + namespacerBin)
	}, func(data []byte) {
		var err error
		binaries := strings.Split(string(data), ":")
		GrootFSBin = string(binaries[0])
		TardisBin = string(binaries[1])
		tmpNamespacerBin := string(binaries[2])

		rand.Seed(time.Now().UnixNano())
		NamespacerBin = fmt.Sprintf("/tmp/namespacer-%d", rand.Int())
		_, _, err = runCommand(exec.Command("cp", tmpNamespacerBin, NamespacerBin))
		Expect(err).NotTo(HaveOccurred())

		grootUser, err := user.Lookup(os.Getenv("GROOTFS_USER"))
		Expect(err).NotTo(HaveOccurred())

		GrootUsername = grootUser.Username

		GrootUID, err = strconv.Atoi(grootUser.Uid)
		Expect(err).NotTo(HaveOccurred())

		GrootGID, err = strconv.Atoi(grootUser.Gid)
		Expect(err).NotTo(HaveOccurred())

		GrootfsTestUid, _ = strconv.Atoi(os.Getenv("GROOTFS_TEST_UID"))
		GrootfsTestGid, _ = strconv.Atoi(os.Getenv("GROOTFS_TEST_GID"))

		fmt.Fprintf(os.Stderr, "============> RUNNING %s TESTS (%d:%d) <=============", "OVERLAY-XFS", GrootfsTestUid, GrootfsTestGid)
	})

	SynchronizedAfterSuite(func() {
	}, func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		testhelpers.ReseedRandomNumberGenerator()

		var err error
		StorePath, err = ioutil.TempDir("", fmt.Sprintf("store-%d", GinkgoParallelNode()))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chmod(StorePath, 0777)).To(Succeed())

		RegistryUsername = os.Getenv("DOCKER_REGISTRY_USERNAME")
		RegistryPassword = os.Getenv("DOCKER_REGISTRY_PASSWORD")

		Runner = runner.Runner{
			GrootFSBin: GrootFSBin,
			StorePath:  StorePath,
			TardisBin:  TardisBin,
		}.WithLogLevel(lager.DEBUG).WithStderr(GinkgoWriter).RunningAsUser(GrootfsTestUid, GrootfsTestGid).WithStoreSize(500)
	})

	AfterEach(func() {
		Expect(deleteAllImages(Runner)).To(Succeed())
		testhelpers.CleanUpOverlayMounts(StorePath)
		Expect(os.RemoveAll(fmt.Sprintf("%s.backing-store", StorePath))).To(Succeed())
		Expect(os.RemoveAll(StorePath)).To(Succeed())
	})

	RunSpecs(t, "Integration Suite")
}

func writeMegabytes(outputPath string, mb int) error {
	_, stderr, err := runCommand(exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", outputPath), "bs=1048576", fmt.Sprintf("count=%d", mb)))
	if err != nil {
		return errors.New(stderr)
	}
	return nil
}

func mountByDefault() bool {
	return GrootfsTestUid == 0
}

func runCommand(command *exec.Cmd) (string, string, error) {
	stdout, stderr := gbytes.NewBuffer(), gbytes.NewBuffer()
	command.Stdout = io.MultiWriter(GinkgoWriter, stdout)
	command.Stderr = io.MultiWriter(GinkgoWriter, stderr)
	err := command.Run()
	return string(stdout.Contents()), string(stderr.Contents()), err
}

func getVolumesDirEntries() ([]os.FileInfo, error) {
	return ioutil.ReadDir(filepath.Join(StorePath, store.VolumesDirName))
}

func deleteAllImages(runner runner.Runner) error {
	imagesPath := filepath.Join(runner.StorePath, "images")
	_, err := os.Stat(imagesPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat %s: %v", imagesPath, err)
	}

	rootfulRunner := runner.RunningAsUser(0, 0)
	images, err := rootfulRunner.List()
	if err != nil {
		return fmt.Errorf("failed to list images: %v", err)
	}

	for _, img := range images {
		imageId := filepath.Base(img.Path)
		if err := rootfulRunner.Delete(imageId); err != nil {
			return fmt.Errorf("failed to delete image %q: %v", imageId, err)
		}
	}

	return nil
}

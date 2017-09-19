package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/lager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Delete (btrfs only)", func() {
	var (
		nextUniqueImageIndex uint32
	)

	BeforeEach(func() {
		integration.SkipIfNotBTRFS(Driver)
	})

	createUniqueImage := func(baseImagePath string) (imageId string, containerSpec specs.Spec) {
		imageId = fmt.Sprintf("image-%d", atomic.AddUint32(&nextUniqueImageIndex, 1))
		containerSpec, err := Runner.Create(groot.CreateSpec{
			BaseImage: baseImagePath,
			ID:        imageId,
			Mount:     true,
		})
		Expect(err).ToNot(HaveOccurred())

		return
	}

	withUniqueImage := func(testFunc func(imageId string, containerSpec specs.Spec)) {
		sourceImagePath, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(os.RemoveAll(sourceImagePath)).To(Succeed()) }()
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())

		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath := baseImageFile.Name()
		defer func() { Expect(os.RemoveAll(baseImagePath)).To(Succeed()) }()

		imageId, containerSpec := createUniqueImage(baseImagePath)
		testFunc(imageId, containerSpec)
	}

	It("destroys the quota group associated with the volume", func() {
		withUniqueImage(func(imageId string, containerSpec specs.Spec) {
			rootIDBuffer := gbytes.NewBuffer()
			sess, err := gexec.Start(exec.Command("sudo", "btrfs", "inspect-internal", "rootid", containerSpec.Root.Path), rootIDBuffer, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
			rootID := strings.TrimSpace(string(rootIDBuffer.Contents()))

			sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", StorePath), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
			Expect(sess).To(gbytes.Say(rootID))

			Expect(Runner.Delete(imageId)).To(Succeed())

			sess, err = gexec.Start(exec.Command("sudo", "btrfs", "qgroup", "show", StorePath), GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))
			Expect(sess).ToNot(gbytes.Say(rootID))
		})
	})

	It("can delete a rootfs with nested btrfs subvolumes", func() {
		withUniqueImage(func(imageId string, containerSpec specs.Spec) {
			cmd := exec.Command("btrfs", "sub", "create", filepath.Join(containerSpec.Root.Path, "subvolume"))
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(GrootfsTestUid),
					Gid: uint32(GrootfsTestGid),
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))

			cmd = exec.Command("btrfs", "sub", "snapshot", filepath.Join(containerSpec.Root.Path, "subvolume"), filepath.Join(containerSpec.Root.Path, "snapshot"))
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(GrootfsTestUid),
					Gid: uint32(GrootfsTestGid),
				},
			}
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))

			Expect(containerSpec.Root.Path).To(BeAnExistingFile())
			Expect(Runner.Delete(imageId)).To(Succeed())
			Expect(containerSpec.Root.Path).ToNot(BeAnExistingFile())
		})
	})

	It("returns a warning when drax is not in the PATH", func() {
		withUniqueImage(func(imageId string, containerSpec specs.Spec) {
			errBuffer := gbytes.NewBuffer()
			outBuffer := gbytes.NewBuffer()
			err := Runner.WithoutDraxBin().
				WithLogLevel(lager.INFO).
				WithEnvVar("PATH=/usr/sbin:/usr/bin:/sbin:/bin").
				WithStdout(outBuffer).
				WithStderr(errBuffer).
				Delete(imageId)
			Expect(err).NotTo(HaveOccurred())
			Eventually(errBuffer).Should(gbytes.Say("could not delete quota group"))
			Eventually(outBuffer).Should(gbytes.Say(fmt.Sprintf("Image %s deleted", imageId)))
		})
	})
})

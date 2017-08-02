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
)

var _ = Describe("Delete (btrfs only)", func() {
	var (
		nextUniqueImageIndex uint32
	)

	BeforeEach(func() {
		integration.SkipIfNotBTRFS(Driver)
	})

	createUniqueImage := func(baseImagePath string) (imageId string, image groot.ImageInfo) {
		imageId = fmt.Sprintf("image-%d", atomic.AddUint32(&nextUniqueImageIndex, 1))
		image, err := Runner.Create(groot.CreateSpec{
			BaseImage: baseImagePath,
			ID:        imageId,
			Mount:     true,
		})
		Expect(err).ToNot(HaveOccurred())

		return
	}

	withUniqueImage := func(testFunc func(imageId string, image groot.ImageInfo)) {
		sourceImagePath, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(os.RemoveAll(sourceImagePath)).To(Succeed()) }()
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "foo"), []byte("hello-world"), 0644)).To(Succeed())

		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath := baseImageFile.Name()
		defer func() { Expect(os.RemoveAll(baseImagePath)).To(Succeed()) }()

		imageId, image := createUniqueImage(baseImagePath)
		testFunc(imageId, image)
	}

	It("destroys the quota group associated with the volume", func() {
		withUniqueImage(func(imageId string, image groot.ImageInfo) {
			rootIDBuffer := gbytes.NewBuffer()
			sess, err := gexec.Start(exec.Command("sudo", "btrfs", "inspect-internal", "rootid", image.Rootfs), rootIDBuffer, GinkgoWriter)
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
		withUniqueImage(func(imageId string, image groot.ImageInfo) {
			cmd := exec.Command("btrfs", "sub", "create", filepath.Join(image.Rootfs, "subvolume"))
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(GrootfsTestUid),
					Gid: uint32(GrootfsTestGid),
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))

			cmd = exec.Command("btrfs", "sub", "snapshot", filepath.Join(image.Rootfs, "subvolume"), filepath.Join(image.Rootfs, "snapshot"))
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: uint32(GrootfsTestUid),
					Gid: uint32(GrootfsTestGid),
				},
			}
			sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 5*time.Second).Should(gexec.Exit(0))

			Expect(image.Rootfs).To(BeAnExistingFile())
			Expect(Runner.Delete(imageId)).To(Succeed())
			Expect(image.Rootfs).ToNot(BeAnExistingFile())
		})
	})

	It("returns a warning when drax is not in the PATH", func() {
		withUniqueImage(func(imageId string, image groot.ImageInfo) {
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

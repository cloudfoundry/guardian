package root_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create", func() {
	var (
		baseImagePath string
		rootUID       int
		rootGID       int
	)

	BeforeEach(func() {
		rootUID = 0
		rootGID = 0

		var err error
		baseImagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chown(baseImagePath, rootUID, rootGID)).To(Succeed())
		Expect(os.Chmod(baseImagePath, 0755)).To(Succeed())

		grootFilePath := path.Join(baseImagePath, "foo")
		Expect(ioutil.WriteFile(grootFilePath, []byte("hello-world"), 0644)).To(Succeed())
		Expect(os.Chown(grootFilePath, int(GrootUID), int(GrootGID))).To(Succeed())

		grootFolder := path.Join(baseImagePath, "groot-folder")
		Expect(os.Mkdir(grootFolder, 0777)).To(Succeed())
		Expect(os.Chown(grootFolder, int(GrootUID), int(GrootGID))).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(grootFolder, "hello"), []byte("hello-world"), 0644)).To(Succeed())

		rootFilePath := path.Join(baseImagePath, "bar")
		Expect(ioutil.WriteFile(rootFilePath, []byte("hello-world"), 0644)).To(Succeed())

		rootFolder := path.Join(baseImagePath, "root-folder")
		Expect(os.Mkdir(rootFolder, 0777)).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(rootFolder, "hello"), []byte("hello-world"), 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
	})

	It("keeps the ownership and permissions", func() {
		image := integration.CreateImage(GrootFSBin, StorePath, DraxBin, baseImagePath, "random-id", 0)

		grootFi, err := os.Stat(path.Join(image.RootFSPath, "foo"))
		Expect(err).NotTo(HaveOccurred())
		Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
		Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

		rootFi, err := os.Stat(path.Join(image.RootFSPath, "bar"))
		Expect(err).NotTo(HaveOccurred())
		Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(rootUID)))
		Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(rootGID)))
	})

	Context("when mappings are provided", func() {
		// This test is in the root suite not because `grootfs` is run by root, but
		// because we need to write a file as root to test the translation.
		It("translates the rootfs accordingly", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"--log-level", "debug",
				"create",
				"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--uid-mapping", "1:100000:65000",
				"--gid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--gid-mapping", "1:100000:65000",
				baseImagePath,
				"some-id",
			)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess).Should(gexec.Exit(0))
			image := strings.TrimSpace(string(sess.Out.Contents()))

			grootFi, err := os.Stat(path.Join(image, "rootfs", "foo"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			grootDir, err := os.Stat(path.Join(image, "rootfs", "groot-folder"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID + 99999)))
			Expect(grootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID + 99999)))

			rootFi, err := os.Stat(path.Join(image, "rootfs", "bar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))

			rootDir, err := os.Stat(path.Join(image, "rootfs", "root-folder"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootDir.Sys().(*syscall.Stat_t).Uid).To(Equal(uint32(GrootUID)))
			Expect(rootDir.Sys().(*syscall.Stat_t).Gid).To(Equal(uint32(GrootGID)))
		})
	})

	Context("when image is local", func() {
		It("logs the steps taken to create the rootfs", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"--log-level", "debug",
				"create",
				"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--uid-mapping", "1:100000:65000",
				"--gid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--gid-mapping", "1:100000:65000",
				baseImagePath,
				"some-id",
			)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 10*time.Second).Should(gexec.Exit(0))

			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.namespaced-unpacking.starting-unpack-wrapper-command"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.namespaced-unpacking.mapUID.starting-id-map"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.namespaced-unpacking.mapGID.starting-id-map"))
		})
	})

	Context("when image is remote", func() {
		It("logs the steps taken to create the rootfs", func() {
			cmd := exec.Command(
				GrootFSBin, "--store", StorePath,
				"--log-level", "debug",
				"create",
				"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--uid-mapping", "1:100000:65000",
				"--gid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--gid-mapping", "1:100000:65000",
				"docker:///cfgarden/empty:v0.1.0",
				"some-id",
			)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: GrootUID,
					Gid: GrootGID,
				},
			}
			sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())
			Eventually(sess, 10*time.Second).Should(gexec.Exit(0))

			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.btrfs-creating-volume.starting-btrfs"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.namespaced-unpacking.starting-unpack-wrapper-command"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.namespaced-unpacking.mapUID.starting-id-map"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.namespaced-unpacking.mapGID.starting-id-map"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.image-pulling.namespaced-unpacking.unpack-wrapper.starting-unpack"))
			Eventually(sess.Err).Should(gbytes.Say("grootfs.create.groot-creating.making-image.btrfs-creating-snapshot.starting-btrfs"))
		})
	})
})

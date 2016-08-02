package root_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"

	"code.cloudfoundry.org/grootfs/integration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Create", func() {
	var (
		imagePath string
		rootUID   int
		rootGID   int
	)

	BeforeEach(func() {
		rootUID = 0
		rootGID = 0

		var err error
		imagePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chown(imagePath, rootUID, rootGID)).To(Succeed())
		Expect(os.Chmod(imagePath, 0755)).To(Succeed())

		Expect(ioutil.WriteFile(path.Join(imagePath, "foo"), []byte("hello-world"), 0700)).To(Succeed())
		Expect(os.Chown(path.Join(imagePath, "foo"), rootUID, rootGID)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	It("keeps the ownership and permissions", func() {
		bundle := integration.CreateBundle(GrootFSBin, GraphPath, imagePath, "random-id")

		stat, err := os.Stat(path.Join(bundle.RootFsPath(), "foo"))
		Expect(err).NotTo(HaveOccurred())
		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(BeEquivalentTo(rootUID))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(BeEquivalentTo(rootGID))
	})

	Context("when a uid mapping is provided", func() {
		// This test is in the root suite not because `grootfs` is run by root, but
		// because we need to write a file as root to test the translation.
		It("should translate the rootfs accordingly", func() {
			grootFilePath := path.Join(imagePath, "foo")
			Expect(ioutil.WriteFile(grootFilePath, []byte("hello-world"), 0644)).To(Succeed())
			Expect(os.Chown(grootFilePath, int(GrootUID), int(GrootGID))).To(Succeed())
			rootFilePath := path.Join(imagePath, "bar")
			Expect(ioutil.WriteFile(rootFilePath, []byte("hello-world"), 0644)).To(Succeed())

			cmd := exec.Command(
				GrootFSBin, "--graph", GraphPath,
				"create", "--image", imagePath,
				"--uid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--uid-mapping", "1:100000:65000",
				"--gid-mapping", fmt.Sprintf("0:%d:1", GrootUID),
				"--gid-mapping", "1:100000:65000",
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
			bundle := strings.TrimSpace(string(sess.Out.Contents()))

			grootFi, err := os.Stat(path.Join(bundle, "rootfs", "foo"))
			Expect(err).NotTo(HaveOccurred())
			Expect(grootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(GrootUID + 99999))
			Expect(grootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(GrootGID + 99999))

			rootFi, err := os.Stat(path.Join(bundle, "rootfs", "bar"))
			Expect(err).NotTo(HaveOccurred())
			Expect(rootFi.Sys().(*syscall.Stat_t).Uid).To(Equal(GrootUID))
			Expect(rootFi.Sys().(*syscall.Stat_t).Gid).To(Equal(GrootGID))
		})
	})
})

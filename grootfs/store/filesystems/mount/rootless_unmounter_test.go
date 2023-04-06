package mount_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/store/filesystems/mount"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rootless Unmounter", func() {
	var (
		tmpDir        string
		mountDestPath string
		mountSrcPath  string

		unmounter  mount.RootlessUnmounter
		unmountErr error
		logger     lager.Logger
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		mountSrcPath = filepath.Join(tmpDir, "mntsrc")
		Expect(os.MkdirAll(mountSrcPath, 755)).To(Succeed())

		mountDestPath = filepath.Join(tmpDir, "mntdest")
		Expect(os.MkdirAll(mountDestPath, 755)).To(Succeed())

		unmounter = mount.RootlessUnmounter{}
		logger = lagertest.NewTestLogger("rootless-unmounter")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		unmountErr = unmounter.Unmount(logger, mountDestPath)
	})

	When("the directory to unmount is mounted", func() {
		BeforeEach(func() {
			Expect(exec.Command("mount", "--bind", mountSrcPath, mountDestPath).Run()).To(Succeed())
		})

		AfterEach(func() {
			Expect(syscall.Unmount(mountDestPath, 0)).To(Succeed())
		})

		It("returns an error", func() {
			Expect(unmountErr).To(MatchError(ContainSubstring("cannot be unmounted when running rootless")))
		})
	})

	When("the directory to unmount is not mounted", func() {
		It("does not error", func() {
			Expect(unmountErr).NotTo(HaveOccurred())
		})
	})

	When("the directory to unmount does not exist", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(mountDestPath)).To(Succeed())
		})

		It("does not error", func() {
			Expect(unmountErr).NotTo(HaveOccurred())
		})
	})
})

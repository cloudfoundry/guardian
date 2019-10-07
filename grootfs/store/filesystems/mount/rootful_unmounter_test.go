package mount_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/grootfs/store/filesystems/mount"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/sys/unix"
)

var _ = Describe("Rootful Unmounter", func() {
	var (
		tmpDir        string
		mountDestPath string
		mountSrcPath  string

		unmounter  mount.RootfulUnmounter
		unmountErr error
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		mountSrcPath = filepath.Join(tmpDir, "mntsrc")
		Expect(os.MkdirAll(mountSrcPath, 755)).To(Succeed())

		mountDestPath = filepath.Join(tmpDir, "mntdest")
		Expect(os.MkdirAll(mountDestPath, 755)).To(Succeed())

		unmounter = mount.RootfulUnmounter{}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		unmountErr = unmounter.Unmount(mountDestPath)
	})

	It("is rootful", func() {
		Expect(unmounter.IsRootless()).To(BeFalse())
	})

	When("the directory to unmount is mounted", func() {
		BeforeEach(func() {
			Expect(exec.Command("mount", "--bind", mountSrcPath, mountDestPath).Run()).To(Succeed())
		})

		AfterEach(func() {
			err := syscall.Unmount(mountDestPath, 0)
			// do not fail if not mounted
			if err != unix.EINVAL {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("unmounts it", func() {
			Expect(unmountErr).NotTo(HaveOccurred())
			mountTable, err := ioutil.ReadFile("/proc/self/mountinfo")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(mountTable)).NotTo(ContainSubstring(mountDestPath))
		})

		When("unmount fails", func() {
			var busyFile *os.File

			BeforeEach(func() {
				var err error
				busyFile, err = os.Create(filepath.Join(mountDestPath, "busyfile"))
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(busyFile.Close()).To(Succeed())
			})

			It("fails with busy error", func() {
				Expect(unmountErr).To(MatchError(unix.EBUSY))
			})
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

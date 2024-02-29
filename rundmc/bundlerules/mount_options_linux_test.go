package bundlerules_test

import (
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/bundlerules"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mount options", func() {
	var (
		mountPoint         string
		mountOptions       []string
		getMountOptionsErr error
		tmpDir             string
	)

	JustBeforeEach(func() {
		mountOptions, getMountOptionsErr = bundlerules.UnprivilegedMountFlagsGetter(mountPoint)
	})

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "mountpoint-options")
		Expect(err).NotTo(HaveOccurred())
		mountPoint = filepath.Join(tmpDir, "mount-point")
		Expect(os.Mkdir(mountPoint, os.ModeDir)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Context("when the path is a file bind mount", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(mountPoint)).To(Succeed())
			_, err := os.Create(mountPoint)
			Expect(err).NotTo(HaveOccurred())

			Expect(exec.Command("mount", "--bind", mountPoint, mountPoint).Run()).To(Succeed())
			Expect(exec.Command("mount", "-o", "remount,noexec,bind", mountPoint, mountPoint).Run()).To(Succeed())
		})

		AfterEach(func() {
			cmd := exec.Command("umount", mountPoint)
			Expect(cmd.Run()).To(Succeed())
		})

		It("returns mount options", func() {
			Expect(getMountOptionsErr).NotTo(HaveOccurred())
			Expect(mountOptions).To(SatisfyAll(ContainElement("noexec")))
		})
	})

	Context("when the path is not a mountpoint", func() {
		It("returns an empty options list", func() {
			Expect(getMountOptionsErr).NotTo(HaveOccurred())
			Expect(mountOptions).To(BeEmpty())
		})
	})

	Context("when the path is a mount point", func() {
		BeforeEach(func() {
			toBeMounted := filepath.Join(tmpDir, "to-be-mounted")
			Expect(os.Mkdir(toBeMounted, os.ModeDir)).To(Succeed())

			cmd := exec.Command("mount", "-t", "tmpfs", "-o", "ro,noexec", toBeMounted, mountPoint)
			Expect(cmd.Run()).To(Succeed())
		})

		AfterEach(func() {
			cmd := exec.Command("umount", mountPoint)
			Expect(cmd.Run()).To(Succeed())
		})

		It("returns the mount options", func() {
			Expect(mountOptions).To(SatisfyAll(ContainElement("ro"), ContainElement("noexec")))
		})
	})
})

package rundmc_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mount info", func() {
	var (
		mountPoint  string
		toBeMounted string
		tmpDir      string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "mountpoint-info")
		Expect(err).NotTo(HaveOccurred())

		toBeMounted = filepath.Join(tmpDir, "to-be-mounted")
		Expect(os.Mkdir(toBeMounted, os.ModeDir)).To(Succeed())

		mountPoint = filepath.Join(tmpDir, "mount-point")
		Expect(os.Mkdir(mountPoint, os.ModeDir)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Describe("Checking whether is a mount point", func() {
		var (
			isMountPoint bool
			isMountError error
		)

		JustBeforeEach(func() {
			isMountPoint, isMountError = rundmc.IsMountPoint(mountPoint)
		})

		Context("is a mountpoint", func() {
			BeforeEach(func() {
				cmd := exec.Command("mount", "-t", "tmpfs", toBeMounted, mountPoint)
				Expect(cmd.Run()).To(Succeed())
			})

			AfterEach(func() {
				cmd := exec.Command("umount", mountPoint)
				Expect(cmd.Run()).To(Succeed())
			})

			It("reports it as mountpoint", func() {
				Expect(isMountError).NotTo(HaveOccurred())
				Expect(isMountPoint).To(BeTrue())
			})
		})

		Context("not a mountpoint", func() {

			It("does not report it as a mountpoint", func() {
				Expect(isMountError).NotTo(HaveOccurred())
				Expect(isMountPoint).To(BeFalse())
			})
		})

		Context("when the path does not exist", func() {
			BeforeEach(func() {
				mountPoint = "/does/not/exist"
			})

			It("does not report it as a mountpoint", func() {
				Expect(isMountError).NotTo(HaveOccurred())
				Expect(isMountPoint).To(BeFalse())
			})
		})
	})

	Describe("Getting mount options", func() {
		var (
			mountOptions       []string
			getMountOptionsErr error
		)

		JustBeforeEach(func() {
			mountOptions, getMountOptionsErr = rundmc.GetMountOptions(mountPoint)
		})

		Context("when the path does not exist", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(mountPoint)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(getMountOptionsErr).To(HaveOccurred())
			})
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

		Context("when the path is not mounted", func() {
			It("returns an informative error", func() {
				Expect(getMountOptionsErr).To(HaveOccurred())
				Expect(getMountOptionsErr.Error()).To(ContainSubstring("not a mount point"))
			})
		})

		Context("when the path is a mount point", func() {
			BeforeEach(func() {
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
})

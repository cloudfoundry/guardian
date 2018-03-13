package cgroups_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/cgroups"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mountpoint checker", func() {

	var (
		mountPoint   string
		toBeMounted  string
		tmpDir       string
		isMountPoint bool
		isMountError error
	)

	JustBeforeEach(func() {
		isMountPoint, isMountError = cgroups.IsMountPoint(mountPoint)
	})

	Context("when the path exists", func() {
		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "mountpoint-checker")
			Expect(err).NotTo(HaveOccurred())

			mountPoint = filepath.Join(tmpDir, "mount-point")
			Expect(os.Mkdir(mountPoint, os.ModeDir)).To(Succeed())

		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		Context("and is a mountpoint", func() {

			BeforeEach(func() {

				toBeMounted = filepath.Join(tmpDir, "to-be-mounted")
				Expect(os.Mkdir(toBeMounted, os.ModeDir)).To(Succeed())

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

		Context("when the path in not a mountpoint", func() {

			It("does not report it as a mountpoint", func() {
				Expect(isMountError).NotTo(HaveOccurred())
				Expect(isMountPoint).To(BeFalse())
			})
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

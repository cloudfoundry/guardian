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

var _ = Describe("Mount options", func() {
	var (
		mountPoint         string
		mountOptions       []string
		toBeMounted        string
		getMountOptionsErr error
		tmpDir             string
	)

	JustBeforeEach(func() {
		mountOptions, getMountOptionsErr = rundmc.GetMountOptions(mountPoint)
	})

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "mountpoint-options")
		Expect(err).NotTo(HaveOccurred())

		toBeMounted = filepath.Join(tmpDir, "to-be-mounted")
		Expect(os.Mkdir(toBeMounted, os.ModeDir)).To(Succeed())

		mountPoint = filepath.Join(tmpDir, "mount-point")
		Expect(os.Mkdir(mountPoint, os.ModeDir)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	Context("when the path does not exist", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(mountPoint)).To(Succeed())
		})

		It("returns an error", func() {
			Expect(getMountOptionsErr).To(HaveOccurred())
		})
	})

	Context("when the path is not a directory", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(mountPoint)).To(Succeed())
			_, err := os.Create(mountPoint)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an informative error", func() {
			Expect(getMountOptionsErr).To(HaveOccurred())
			Expect(getMountOptionsErr.Error()).To(ContainSubstring("not a directory"))
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

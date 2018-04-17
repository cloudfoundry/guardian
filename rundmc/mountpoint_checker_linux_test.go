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

var _ = Describe("Mountpoint checker", func() {

	var (
		pathToCheck  string
		toBeMounted  string
		tmpDir       string
		isMountPoint bool
		isMountError error
	)

	JustBeforeEach(func() {
		isMountPoint, isMountError = rundmc.IsMountPoint(pathToCheck)
	})

	Context("when the path exists", func() {
		BeforeEach(func() {
			var err error
			tmpDir, err = ioutil.TempDir("", "mountpoint-checker")
			Expect(err).NotTo(HaveOccurred())

			pathToCheck = filepath.Join(tmpDir, "mount-point")
			Expect(os.Mkdir(pathToCheck, os.ModeDir)).To(Succeed())

		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		Context("and is a mountpoint", func() {

			BeforeEach(func() {

				toBeMounted = filepath.Join(tmpDir, "to-be-mounted")
				Expect(os.Mkdir(toBeMounted, os.ModeDir)).To(Succeed())

				cmd := exec.Command("mount", "-t", "tmpfs", toBeMounted, pathToCheck)
				Expect(cmd.Run()).To(Succeed())
			})

			AfterEach(func() {
				cmd := exec.Command("umount", pathToCheck)
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
			pathToCheck = "/does/not/exist"
		})

		It("does not report it as a mountpoint", func() {
			Expect(isMountError).NotTo(HaveOccurred())
			Expect(isMountPoint).To(BeFalse())
		})
	})

	Context("when the path is a file", func() {
		BeforeEach(func() {
			file, err := ioutil.TempFile("", "not-a-dir")
			Expect(err).NotTo(HaveOccurred())
			defer file.Close()
			pathToCheck = file.Name()
		})

		AfterEach(func() {
			Expect(os.Remove(pathToCheck)).To(Succeed())
		})

		It("does not report it as a mountpoint", func() {
			Expect(isMountError).NotTo(HaveOccurred())
			Expect(isMountPoint).To(BeFalse())
		})
	})
})

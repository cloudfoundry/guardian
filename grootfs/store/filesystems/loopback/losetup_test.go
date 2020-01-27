package loopback_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"code.cloudfoundry.org/grootfs/store/filesystems/loopback"
	"code.cloudfoundry.org/grootfs/testhelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LoSetupWrapper", func() {
	var (
		backingFilePath      string
		associatedLoopDevice string

		loSetup loopback.LoSetupWrapper
	)

	BeforeEach(func() {
		f, err := ioutil.TempFile("", "backing-file*")
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()

		backingFilePath = f.Name()

		Expect(os.Truncate(backingFilePath, 1*1024*1024)).To(Succeed())

		losetupCmd := exec.Command("losetup", "--show", "-f", backingFilePath)
		losetupOut, err := losetupCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("losetup output: %q", string(losetupOut)))

		associatedLoopDevice = strings.TrimSpace(string(losetupOut))
	})

	AfterEach(func() {
		losetupCmd := exec.Command("losetup", "-d", associatedLoopDevice)
		losetupOut, err := losetupCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("losetup output: %q", string(losetupOut)))

		Expect(os.RemoveAll(backingFilePath)).To(Succeed())
	})

	Describe("FindAssociatedLoopDevice", func() {
		var (
			foundAssociatedLoopDevice string
			findErr                   error
			fsPath                    string
		)

		BeforeEach(func() {
			fsPath = backingFilePath
		})

		JustBeforeEach(func() {
			foundAssociatedLoopDevice, findErr = loSetup.FindAssociatedLoopDevice(fsPath)
		})

		It("finds the associated loopback device", func() {
			Expect(findErr).NotTo(HaveOccurred())
			Expect(foundAssociatedLoopDevice).To(Equal(associatedLoopDevice))
		})

		When("the file system path does not exist", func() {
			BeforeEach(func() {
				fsPath = "/does/not/exist"
			})

			It("returns a not exist error", func() {
				Expect(os.IsNotExist(findErr)).To(BeTrue())
			})
		})

		When("there is no associated loopback device", func() {
			BeforeEach(func() {
				f, err := ioutil.TempFile("", "another-file*")
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()

				fsPath = f.Name()
			})

			AfterEach(func() {
				Expect(os.RemoveAll(fsPath)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(findErr).To(MatchError(ContainSubstring("no loop device")))
			})
		})
	})

	Describe("EnableDirectIO", func() {
		var (
			loopDevPath string
			enableErr   error
		)

		BeforeEach(func() {
			loopDevPath = associatedLoopDevice
		})

		JustBeforeEach(func() {
			enableErr = loSetup.EnableDirectIO(loopDevPath)
		})

		It("enables direct IO on a loopback device", func() {
			Expect(enableErr).NotTo(HaveOccurred())
			isDirectIO, err := testhelpers.IsDirectIOEnabled(loopDevPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(isDirectIO).To(BeTrue())
		})

		When("directIO is already enabled", func() {
			BeforeEach(func() {
				Expect(loSetup.EnableDirectIO(loopDevPath)).To(Succeed())
			})

			It("does not fail", func() {
				Expect(enableErr).NotTo(HaveOccurred())
			})

			It("does not change the direct-io state", func() {
				isDirectIO, err := testhelpers.IsDirectIOEnabled(loopDevPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDirectIO).To(BeTrue())
			})
		})

		When("the file system path does not exist", func() {
			BeforeEach(func() {
				loopDevPath = "/does/not/exist"
			})

			It("returns a not exist error", func() {
				Expect(os.IsNotExist(enableErr)).To(BeTrue())
			})
		})

		When("there is no associated loopback device", func() {
			BeforeEach(func() {
				f, err := ioutil.TempFile("", "another-file*")
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()

				loopDevPath = f.Name()
			})

			AfterEach(func() {
				Expect(os.RemoveAll(loopDevPath)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(enableErr).To(MatchError(ContainSubstring("failed to set direct-io")))
			})
		})
	})

	Describe("DisableDirectIO", func() {
		var (
			loopDevPath string
			enableErr   error
		)

		BeforeEach(func() {
			loopDevPath = associatedLoopDevice
			enableErr = loSetup.EnableDirectIO(loopDevPath)
		})

		JustBeforeEach(func() {
			enableErr = loSetup.DisableDirectIO(loopDevPath)
		})

		It("disables direct IO on a loopback device", func() {
			Expect(enableErr).NotTo(HaveOccurred())
			isDirectIO, err := testhelpers.IsDirectIOEnabled(loopDevPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(isDirectIO).To(BeFalse())
		})

		When("directIO is already disabled", func() {
			BeforeEach(func() {
				Expect(loSetup.DisableDirectIO(loopDevPath)).To(Succeed())
			})

			It("does not fail", func() {
				Expect(enableErr).NotTo(HaveOccurred())
			})

			It("does not change the direct-io state", func() {
				isDirectIO, err := testhelpers.IsDirectIOEnabled(loopDevPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(isDirectIO).To(BeFalse())
			})
		})

		When("the file system path does not exist", func() {
			BeforeEach(func() {
				loopDevPath = "/does/not/exist"
			})

			It("returns a not exist error", func() {
				Expect(os.IsNotExist(enableErr)).To(BeTrue())
			})
		})

		When("there is no associated loopback device", func() {
			BeforeEach(func() {
				f, err := ioutil.TempFile("", "another-file*")
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()

				loopDevPath = f.Name()
			})

			AfterEach(func() {
				Expect(os.RemoveAll(loopDevPath)).To(Succeed())
			})

			It("returns an error", func() {
				Expect(enableErr).To(MatchError(ContainSubstring("failed to set direct-io")))
			})
		})
	})
})

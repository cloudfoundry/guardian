package peas_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/peas/peasfakes"
	"code.cloudfoundry.org/guardian/rundmc/peas/processwaiter/processwaiterfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("PeaCleaner", func() {
	var (
		fakeRuncDeleter *peasfakes.FakeRuncDeleter
		fakeVolumizer   *peasfakes.FakeVolumizer
		cleaner         gardener.PeaCleaner
		logger          *lagertest.TestLogger
		processID       string = "proccess-id"
		cleanErr        error
	)

	BeforeEach(func() {
		fakeRuncDeleter = new(peasfakes.FakeRuncDeleter)
		fakeVolumizer = new(peasfakes.FakeVolumizer)
		logger = lagertest.NewTestLogger("peas-unit-tests")
	})

	Describe("Clean", func() {

		JustBeforeEach(func() {
			cleaner = &peas.PeaCleaner{
				RuncDeleter: fakeRuncDeleter,
				Volumizer:   fakeVolumizer,
			}
			cleanErr = cleaner.Clean(logger, processID)
		})

		It("deletes the container", func() {
			Expect(fakeRuncDeleter.DeleteCallCount()).To(Equal(1))
			_, force, id := fakeRuncDeleter.DeleteArgsForCall(0)
			Expect(force).To(BeTrue())
			Expect(id).To(Equal(processID))
		})

		Context("when deleting container fails", func() {
			BeforeEach(func() {
				fakeRuncDeleter.DeleteReturns(errors.New("failky"))
			})

			It("returns an error", func() {
				Expect(cleanErr).To(MatchError(ContainSubstring("failky")))
			})

			It("still destroys rootfs", func() {
				Expect(fakeVolumizer.DestroyCallCount()).To(Equal(1))
			})
		})

		It("destroys the volume", func() {
			Expect(fakeVolumizer.DestroyCallCount()).To(Equal(1))
			_, id := fakeVolumizer.DestroyArgsForCall(0)
			Expect(id).To(Equal(processID))
		})

		Context("when deleting container fails", func() {
			BeforeEach(func() {
				fakeVolumizer.DestroyReturns(errors.New("abracadabrakaboom"))
			})

			It("returns an error", func() {
				Expect(cleanErr).To(MatchError(ContainSubstring("abracadabrakaboom")))
			})

		})
	})

	Describe("CleanAll", func() {

		var (
			tmpDir         string
			depotDir       string
			fakeProcWaiter *processwaiterfakes.FakeProcessWaiter
		)

		BeforeEach(func() {
			tmpDir = tempDir()
			depotDir = tmpDir
			mkdirAll(filepath.Join(depotDir, "cake"))
			fakeProcWaiter = new(processwaiterfakes.FakeProcessWaiter)
		})

		JustBeforeEach(func() {
			cleaner = &peas.PeaCleaner{
				RuncDeleter:    fakeRuncDeleter,
				Volumizer:      fakeVolumizer,
				Waiter:         fakeProcWaiter.Spy,
				DepotDirectory: depotDir,
			}
			cleanErr = cleaner.CleanAll(logger)
		})

		AfterEach(func() {
			Expect(os.RemoveAll(tmpDir)).To(Succeed())
		})

		It("does not return an error", func() {
			Expect(cleanErr).NotTo(HaveOccurred())
		})

		It("does not perform cleanup", func() {
			Consistently(fakeRuncDeleter.DeleteCallCount).Should(BeZero())
		})

		Context("when we have an existing pea", func() {
			var (
				peaPath string
			)

			BeforeEach(func() {
				peaPath = filepath.Join(depotDir, "cake", "processes", processID)
				mkdirAll(peaPath)
				writeFile(filepath.Join(peaPath, "config.json"), "")
				writeFile(filepath.Join(peaPath, "pidfile"), "7\n")
			})

			It("does not return an error", func() {
				Expect(cleanErr).NotTo(HaveOccurred())
			})

			Context("when getting the peas fails", func() {
				Context("when the depot dir does not exist", func() {
					BeforeEach(func() {
						Expect(os.RemoveAll(depotDir)).To(Succeed())
					})

					It("returns an error", func() {
						Expect(os.IsNotExist(cleanErr)).To(BeTrue())
					})
				})
			})

			Context("when the pid file does not exist", func() {
				BeforeEach(func() {
					pidFilePath := filepath.Join(depotDir, "cake", "processes", processID, "pidfile")
					Expect(os.RemoveAll(pidFilePath)).To(Succeed())
				})

				It("does not return an error", func() {
					Expect(cleanErr).NotTo(HaveOccurred())
				})
			})

			It("waits for the pea to complete", func() {
				Eventually(fakeProcWaiter.CallCount).Should(Equal(1))
				Expect(fakeProcWaiter.ArgsForCall(0)).To(Equal(7))
			})

			It("cleans up the pea", func() {
				Eventually(fakeVolumizer.DestroyCallCount).Should(Equal(1))
				_, id := fakeVolumizer.DestroyArgsForCall(0)
				Expect(id).To(Equal(processID))
			})

			Context("when we have multiple containers", func() {
				BeforeEach(func() {
					mkdirAll(filepath.Join(depotDir, "potato", "processes", "non-pea"))
				})

				It("does not perform cleanup for non-peas", func() {
					Eventually(fakeRuncDeleter.DeleteCallCount).Should(Equal(1))
					Consistently(fakeRuncDeleter.DeleteCallCount).Should(Equal(1))
				})
			})

			Context("and the second one is also a pea", func() {
				BeforeEach(func() {
					secondPeaPath := filepath.Join(depotDir, "cake", "processes", "pea2")
					mkdirAll(secondPeaPath)
					writeFile(filepath.Join(secondPeaPath, "config.json"), "")
					writeFile(filepath.Join(secondPeaPath, "pidfile"), "26\n")

				})
				It("performs cleanup for all peas", func() {
					Eventually(fakeRuncDeleter.DeleteCallCount).Should(Equal(2))
				})
			})
		})
	})
})

func tempDir() string {
	dir, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())
	return dir
}

func writeFile(path, content string) {
	err := ioutil.WriteFile(path, []byte(content), os.ModePerm)
	Expect(err).NotTo(HaveOccurred())
}

func mkdirAll(path string) {
	Expect(os.MkdirAll(path, os.ModePerm)).To(Succeed())
}

package peas_test

import (
	"errors"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/peas"
	"code.cloudfoundry.org/guardian/rundmc/peas/peasfakes"
	"code.cloudfoundry.org/guardian/rundmc/peas/processwaiter/processwaiterfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PeaCleaner", func() {
	var (
		fakeDeleter      *peasfakes.FakeDeleter
		fakeVolumizer    *peasfakes.FakeVolumizer
		fakeProcWaiter   *processwaiterfakes.FakeProcessWaiter
		fakeRuntime      *peasfakes.FakeRuntime
		fakePeaPidGetter *peasfakes.FakePeaPidGetter
		cleaner          gardener.PeaCleaner
		logger           *lagertest.TestLogger
		processID        = "proccess-id"
		cleanErr         error
	)

	BeforeEach(func() {
		fakeDeleter = new(peasfakes.FakeDeleter)
		fakeVolumizer = new(peasfakes.FakeVolumizer)
		fakeProcWaiter = new(processwaiterfakes.FakeProcessWaiter)
		fakeRuntime = new(peasfakes.FakeRuntime)
		fakePeaPidGetter = new(peasfakes.FakePeaPidGetter)

		cleaner = &peas.PeaCleaner{
			Deleter:      fakeDeleter,
			Volumizer:    fakeVolumizer,
			Waiter:       fakeProcWaiter.Spy,
			Runtime:      fakeRuntime,
			PeaPidGetter: fakePeaPidGetter,
		}
		logger = lagertest.NewTestLogger("peas-unit-tests")
	})

	Describe("Clean", func() {

		JustBeforeEach(func() {
			cleanErr = cleaner.Clean(logger, processID)
		})

		It("deletes the container", func() {
			Expect(fakeDeleter.DeleteCallCount()).To(Equal(1))
			_, id := fakeDeleter.DeleteArgsForCall(0)
			Expect(id).To(Equal(processID))
		})

		Context("when deleting container fails", func() {
			BeforeEach(func() {
				fakeDeleter.DeleteReturns(errors.New("failky"))
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

		BeforeEach(func() {
			fakeRuntime.ContainerHandlesReturns([]string{"container-handle"}, nil)
			fakeRuntime.ContainerPeaHandlesReturns([]string{"pea-00"}, nil)
			fakePeaPidGetter.GetPeaPidReturns(17, nil)
		})

		JustBeforeEach(func() {
			cleanErr = cleaner.CleanAll(logger)
		})

		It("gets all container handles", func() {
			Expect(fakeRuntime.ContainerHandlesCallCount()).To(Equal(1))
		})

		It("gets the peas for the container", func() {
			Expect(fakeRuntime.ContainerPeaHandlesCallCount()).To(Equal(1))

			_, actualSandboxHandle := fakeRuntime.ContainerPeaHandlesArgsForCall(0)
			Expect(actualSandboxHandle).To(Equal("container-handle"))
		})

		It("waits for the pea to complete", func() {
			Eventually(fakeProcWaiter.CallCount).Should(Equal(1))
			Expect(fakeProcWaiter.ArgsForCall(0)).To(Equal(17))
		})

		It("cleans up the pea", func() {
			Eventually(fakeVolumizer.DestroyCallCount).Should(Equal(1))
			_, actualID := fakeVolumizer.DestroyArgsForCall(0)
			Expect(actualID).To(Equal("pea-00"))
		})

		Context("when there is more than one container", func() {
			BeforeEach(func() {
				fakeRuntime.ContainerHandlesReturns([]string{"first", "second"}, nil)
			})

			It("gets the peas for each of the containers", func() {
				Expect(fakeRuntime.ContainerPeaHandlesCallCount()).To(Equal(2))

				_, actualSandboxHandle := fakeRuntime.ContainerPeaHandlesArgsForCall(0)
				Expect(actualSandboxHandle).To(Equal("first"))

				_, actualSandboxHandle = fakeRuntime.ContainerPeaHandlesArgsForCall(1)
				Expect(actualSandboxHandle).To(Equal("second"))
			})

			Context("when getting the peas for a container fails", func() {
				BeforeEach(func() {
					fakeRuntime.ContainerPeaHandlesReturnsOnCall(0, nil, errors.New("BOOM"))
				})

				It("proceeds with the next container", func() {
					Expect(cleanErr).NotTo(HaveOccurred())
					Expect(fakeRuntime.ContainerPeaHandlesCallCount()).To(Equal(2))
				})
			})
		})

		Context("when there are more than one pea", func() {
			BeforeEach(func() {
				fakeRuntime.ContainerPeaHandlesReturnsOnCall(0, []string{"pea-11", "pea-12"}, nil)
			})

			It("gets the PID of each pea", func() {
				Expect(fakePeaPidGetter.GetPeaPidCallCount()).To(Equal(2))

				_, actualSandboxHandle, actualPeaHandle := fakePeaPidGetter.GetPeaPidArgsForCall(0)
				Expect(actualSandboxHandle).To(Equal("container-handle"))
				Expect(actualPeaHandle).To(Equal("pea-11"))

				_, actualSandboxHandle, actualPeaHandle = fakePeaPidGetter.GetPeaPidArgsForCall(1)
				Expect(actualSandboxHandle).To(Equal("container-handle"))
				Expect(actualPeaHandle).To(Equal("pea-12"))
			})

			Context("when getting the pea pid fails", func() {
				BeforeEach(func() {
					fakePeaPidGetter.GetPeaPidReturnsOnCall(0, -1, errors.New("nopid"))
					fakePeaPidGetter.GetPeaPidReturnsOnCall(1, 43, nil)
				})

				It("proceeds with the next pea", func() {
					Expect(cleanErr).NotTo(HaveOccurred())
					Expect(fakePeaPidGetter.GetPeaPidCallCount()).To(Equal(2))
				})

				It("does not try to clean the pea", func() {
					Eventually(fakeProcWaiter.CallCount).Should(Equal(1))
					Expect(fakeProcWaiter.ArgsForCall(0)).To(Equal(43))
					Consistently(fakeProcWaiter.CallCount).Should(Equal(1))
				})
			})
		})

		Context("when getting the container handles fails", func() {
			BeforeEach(func() {
				fakeRuntime.ContainerHandlesReturns(nil, errors.New("faily"))
			})

			It("propagates the error", func() {
				Expect(cleanErr).To(MatchError("faily"))
			})
		})
	})

	Context("when waiting on the pea fails", func() {
		BeforeEach(func() {
			fakeProcWaiter.Returns(errors.New("NOPE"))
		})

		It("doesn't clean it up", func() {
			Consistently(fakeVolumizer.DestroyCallCount).Should(Equal(0))
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

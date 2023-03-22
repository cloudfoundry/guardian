package runcontainerd_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PeaProcess", func() {
	var (
		logger             lager.Logger
		fakeBackingProcess *runcontainerdfakes.FakeBackingProcess
		fakePeaManager     *runcontainerdfakes.FakePeaManager
		fakeVolumizer      *runcontainerdfakes.FakeVolumizer

		peaProcess *runcontainerd.PeaProcess
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-logger")
		fakePeaManager = new(runcontainerdfakes.FakePeaManager)
		fakeVolumizer = new(runcontainerdfakes.FakeVolumizer)
		fakeBackingProcess = new(runcontainerdfakes.FakeBackingProcess)
		fakeBackingProcess.IDReturns("pea-test")
		proc := runcontainerd.NewProcess(logger, fakeBackingProcess, false)
		peaProcess = runcontainerd.NewPeaProcess(logger, *proc, fakePeaManager, fakeVolumizer)

		fakeBackingProcess.IDReturns("pea-test")
	})

	Describe("Wait", func() {
		var err error

		JustBeforeEach(func() {
			_, err = peaProcess.Wait()
		})

		It("kills the pea process", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePeaManager.DeleteCallCount()).To(Equal(1))
			_, containerId := fakePeaManager.DeleteArgsForCall(0)
			Expect(containerId).To(Equal("pea-test"))
		})

		When("killing the pea process fails", func() {
			BeforeEach(func() {
				fakePeaManager.DeleteReturns(errors.New("boom"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("boom"))
			})
		})

		It("deletes the pea process metadata", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePeaManager.RemoveBundleCallCount()).To(Equal(1))
			_, processId := fakePeaManager.RemoveBundleArgsForCall(0)
			Expect(processId).To(Equal("pea-test"))
		})

		When("deleting the pea process metadata fails", func() {
			BeforeEach(func() {
				fakePeaManager.RemoveBundleReturns(errors.New("boom"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("boom"))
			})
		})

		It("destroys the pea image", func() {
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolumizer.DestroyCallCount()).To(Equal(1))
			_, containerId := fakeVolumizer.DestroyArgsForCall(0)
			Expect(containerId).To(Equal("pea-test"))
		})

		When("destroying the pea image fails", func() {
			BeforeEach(func() {
				fakeVolumizer.DestroyReturns(errors.New("destroy-err"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("destroy-err"))
			})
		})
	})
})

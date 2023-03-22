package deleter_test

import (
	"code.cloudfoundry.org/guardian/rundmc"
	"errors"

	"code.cloudfoundry.org/guardian/rundmc/deleter/deleterfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/guardian/rundmc/deleter"
)

var _ = Describe("PeaDeleter", func() {
	var (
		logger         *lagertest.TestLogger
		runtimeDeleter *deleterfakes.FakeRuntimeDeleter
		runtimeStater  *deleterfakes.FakeRuntimeStater

		deleter *Deleter
		err     error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("pea-deleter-test")
		runtimeDeleter = new(deleterfakes.FakeRuntimeDeleter)
		runtimeStater = new(deleterfakes.FakeRuntimeStater)

		deleter = NewDeleter(runtimeStater, runtimeDeleter)
	})

	JustBeforeEach(func() {
		err = deleter.Delete(logger, "my-handle")
	})

	When("status is stopped", func() {
		BeforeEach(func() {
			runtimeStater.StateReturns(rundmc.State{Pid: 123, Status: rundmc.StoppedStatus}, nil)
		})

		It("invokes Delete() without force", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(runtimeDeleter.DeleteCallCount()).To(Equal(1))
			_, handle, force := runtimeDeleter.DeleteArgsForCall(0)
			Expect(handle).To(Equal("my-handle"))
			Expect(force).To(BeFalse())
		})
	})
	When("status is created", func() {
		BeforeEach(func() {
			runtimeStater.StateReturns(rundmc.State{Pid: 123, Status: rundmc.CreatedStatus}, nil)
		})

		It("invokes Delete()", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(runtimeDeleter.DeleteCallCount()).To(Equal(1))
			_, _, force := runtimeDeleter.DeleteArgsForCall(0)
			Expect(force).To(BeFalse())
		})
	})

	When("status is running", func() {
		BeforeEach(func() {
			runtimeStater.StateReturns(rundmc.State{Pid: 123, Status: rundmc.RunningStatus}, nil)
		})

		It("invokes Delete() with force", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(runtimeDeleter.DeleteCallCount()).To(Equal(1))
			_, _, force := runtimeDeleter.DeleteArgsForCall(0)
			Expect(force).To(BeTrue())
		})
	})

	When("status is not one of the acceptable states", func() {
		BeforeEach(func() {
			runtimeStater.StateReturns(rundmc.State{Pid: 123, Status: "unknown"}, nil)
		})

		It("does not delete", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(runtimeDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("getting the state fails", func() {
		BeforeEach(func() {
			runtimeStater.StateReturns(rundmc.State{}, errors.New("oops"))
		})

		It("does not delete and does not return error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(runtimeDeleter.DeleteCallCount()).To(Equal(0))
		})
	})

	When("delete fails", func() {
		BeforeEach(func() {
			runtimeStater.StateReturns(rundmc.State{Pid: 123, Status: rundmc.StoppedStatus}, nil)
			runtimeDeleter.DeleteReturns(errors.New("delete failed"))
		})

		It("returns the error", func() {
			Expect(err).To(MatchError(ContainSubstring("delete failed")))
		})
	})
})

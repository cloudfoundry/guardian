package runcontainerd_test

import (
	"errors"

	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
)

var _ = Describe("NerdDeleter", func() {
	var (
		logger  *lagertest.TestLogger
		runtime *runcontainerdfakes.FakeRuntime
		err     error
		deleter *NerdDeleter
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("nerd-deleter")
		runtime = new(runcontainerdfakes.FakeRuntime)
		deleter = NewDeleter(runtime)
	})

	JustBeforeEach(func() {
		err = deleter.Delete(logger, "some-handle", false)
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("deletes", func() {
		Expect(runtime.DeleteCallCount()).To(Equal(1))
		_, actualHandle := runtime.DeleteArgsForCall(0)
		Expect(actualHandle).To(Equal("some-handle"))
	})

	It("removes the bundle", func() {
		Expect(runtime.RemoveBundleCallCount()).To(Equal(1))
		_, actualHandle := runtime.RemoveBundleArgsForCall(0)
		Expect(actualHandle).To(Equal("some-handle"))
	})

	When("deleting fails", func() {
		BeforeEach(func() {
			runtime.DeleteReturns(errors.New("delete-failed"))
		})

		It("returns the error", func() {
			Expect(err).To(MatchError(ContainSubstring("delete-failed")))
		})

		It("does not remove the bundle", func() {
			Expect(runtime.RemoveBundleCallCount()).To(Equal(0))
		})
	})

	When("removing the bundle fails", func() {
		BeforeEach(func() {
			runtime.DeleteReturns(errors.New("remove-bundle-failed"))
		})

		It("returns the error", func() {
			Expect(err).To(MatchError(ContainSubstring("remove-bundle-failed")))
		})
	})
})

package throttle_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/guardian/throttle/throttlefakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	multierror "github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CompositeRunnable", func() {
	var (
		compositeRunnable throttle.CompositeRunnable
		firstRunnable     *throttlefakes.FakeRunnable
		secondRunnable    *throttlefakes.FakeRunnable
		thirdRunnable     *throttlefakes.FakeRunnable
		logger            *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		firstRunnable = new(throttlefakes.FakeRunnable)
		secondRunnable = new(throttlefakes.FakeRunnable)
		thirdRunnable = new(throttlefakes.FakeRunnable)

		compositeRunnable = throttle.NewCompositeRunnable(
			firstRunnable,
			secondRunnable,
			thirdRunnable,
		)
	})

	Describe("Run", func() {
		var err error

		JustBeforeEach(func() {
			err = compositeRunnable.Run(logger)
		})

		It("does not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("calls Run on each underlying Runnables, in order", func() {
			Expect(firstRunnable.RunCallCount()).To(Equal(1))
			Expect(secondRunnable.RunCallCount()).To(Equal(1))
			Expect(thirdRunnable.RunCallCount()).To(Equal(1))
		})

		It("passes the logger to each underlying Runnables", func() {
			Expect(firstRunnable.RunArgsForCall(0)).To(Equal(logger))
			Expect(secondRunnable.RunArgsForCall(0)).To(Equal(logger))
			Expect(thirdRunnable.RunArgsForCall(0)).To(Equal(logger))
		})

		When("one of underlying the Runnables fails", func() {
			BeforeEach(func() {
				firstRunnable.RunReturns(errors.New("first-run-error"))
				secondRunnable.RunReturns(errors.New("second-run-error"))
			})

			It("returns a multierror with all failures, in order", func() {
				Expect(err).To(HaveOccurred())

				merr, ok := err.(*multierror.Error)
				Expect(ok).To(BeTrue())
				Expect(merr.Errors).To(HaveLen(2))
				Expect(merr.Errors[0]).To(MatchError("first-run-error"))
				Expect(merr.Errors[1]).To(MatchError("second-run-error"))
			})
		})
	})
})

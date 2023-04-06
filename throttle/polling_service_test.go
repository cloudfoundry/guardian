package throttle_test

import (
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"code.cloudfoundry.org/guardian/throttle"
	"code.cloudfoundry.org/guardian/throttle/throttlefakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
)

var _ = Describe("PollingService", func() {
	var (
		service  *throttle.PollingService
		runnable *throttlefakes.FakeRunnable
		ticker   chan time.Time
		logger   *lagertest.TestLogger
	)

	BeforeEach(func() {
		runnable = new(throttlefakes.FakeRunnable)
		ticker = make(chan time.Time)
		logger = lagertest.NewTestLogger("test")
		service = throttle.NewPollingService(logger, runnable, ticker)
		service.Start()
	})

	It("invokes the runner on every tick", func() {
		ticker <- time.Now()
		eventuallyAndConsistently(runnable.RunCallCount, Equal(1))
	})

	When("stopped", func() {
		BeforeEach(func() {
			service.Stop()
			go func() { ticker <- time.Now() }()
		})

		AfterEach(func() {
			<-ticker
		})

		It("does not inoke the runner on subsequent ticks", func() {
			Consistently(runnable.RunCallCount).Should(Equal(0))
		})
	})

	When("stopped while runner is running", func() {
		var (
			done       bool
			inRunnable sync.WaitGroup
		)

		BeforeEach(func() {
			inRunnable = sync.WaitGroup{}
			inRunnable.Add(1)
			runnable.RunStub = func(logger lager.Logger) error {
				inRunnable.Done()
				time.Sleep(time.Second)
				done = true
				return nil
			}
		})

		It("lets the runnable finish", func() {
			ticker <- time.Now()
			inRunnable.Wait()
			service.Stop()

			Expect(done).To(BeTrue())
		})
	})

	When("the runnable fails", func() {
		BeforeEach(func() {
			runnable.RunReturns(errors.New("run-error"))
		})

		It("logs the error and keeps polling", func() {
			ticker <- time.Now()
			ticker <- time.Now()

			Eventually(runnable.RunCallCount).Should(Equal(2))
			Expect(logger.LogMessages()).To(ContainElement("test.failed-to-run-runnable"))
		})
	})
})

func eventuallyAndConsistently(predicate interface{}, shouldMatcher types.GomegaMatcher) {
	Eventually(predicate).Should(shouldMatcher)
	Consistently(predicate).Should(shouldMatcher)
}

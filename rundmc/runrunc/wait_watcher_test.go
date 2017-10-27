package runrunc_test

import (
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WaitWatcher", func() {
	It("calls Wait only once process.Wait returns", func() {
		waiter := new(fakes.FakeWaiter)
		waitReturns := make(chan struct{})
		waiter.WaitStub = func() (int, error) {
			<-waitReturns
			return 0, nil
		}

		runner := new(fakes.FakeRunner)

		watcher := runrunc.Watcher{}
		go watcher.OnExit(lagertest.NewTestLogger("test"), waiter, runner)

		Consistently(runner.RunCallCount).ShouldNot(Equal(1))
		close(waitReturns)
		Eventually(runner.RunCallCount).Should(Equal(1))
	})
})

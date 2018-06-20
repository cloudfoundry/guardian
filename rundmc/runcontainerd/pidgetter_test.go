package runcontainerd_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Pidgetter", func() {
	var (
		pidgetter PidGetter
		logger    lager.Logger
		nerd      *runcontainerdfakes.FakeContainerManager
		pid       int
		pidError  error
	)

	BeforeEach(func() {
		nerd = new(runcontainerdfakes.FakeContainerManager)
		nerd.StateReturns(1234, "", nil)
		pidgetter = PidGetter{Nerd: nerd}
		logger = lagertest.NewTestLogger("banana")
	})

	JustBeforeEach(func() {
		pid, pidError = pidgetter.GetPid(logger, "container-handle")
	})

	It("returns the pid", func() {
		Expect(pid).To(Equal(1234))
	})

	It("gets the container state", func() {
		Expect(nerd.StateCallCount()).To(Equal(1))
		actualLogger, actualContainerID := nerd.StateArgsForCall(0)
		Expect(actualLogger).To(Equal(logger))
		Expect(actualContainerID).To(Equal("container-handle"))
	})

	Context("when getting the container state fails", func() {
		BeforeEach(func() {
			nerd.StateReturns(-1, "", errors.New("get-state-failure"))
		})

		It("returns the error", func() {
			Expect(pidError).To(MatchError("get-state-failure"))
		})
	})
})

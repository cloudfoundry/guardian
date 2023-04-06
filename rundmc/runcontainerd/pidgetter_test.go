package runcontainerd_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
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

	Describe("GetPid", func() {
		JustBeforeEach(func() {
			pid, pidError = pidgetter.GetPid(logger, "container-handle")
		})

		It("gets the container state", func() {
			Expect(nerd.StateCallCount()).To(Equal(1))
			actualLogger, actualContainerID := nerd.StateArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualContainerID).To(Equal("container-handle"))
		})

		It("returns the pid", func() {
			Expect(pid).To(Equal(1234))
		})

		When("getting the container state errors", func() {
			BeforeEach(func() {
				nerd.StateReturns(-1, "", errors.New("get-state-failure"))
			})

			It("returns the error", func() {
				Expect(pidError).To(MatchError("get-state-failure"))
			})
		})
	})

	Describe("GetPeaPid", func() {
		JustBeforeEach(func() {
			pid, pidError = pidgetter.GetPeaPid(logger, "", "pea-id")
		})

		It("gets the pea container state", func() {
			Expect(nerd.StateCallCount()).To(Equal(1))
			actualLogger, actualContainerID := nerd.StateArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualContainerID).To(Equal("pea-id"))
		})

		It("returns the pid", func() {
			Expect(pid).To(Equal(1234))
		})

		When("getting the container state errors", func() {
			BeforeEach(func() {
				nerd.StateReturns(-1, "", errors.New("get-state-failure"))
			})

			It("returns the error", func() {
				Expect(pidError).To(MatchError("get-state-failure"))
			})
		})
	})
})

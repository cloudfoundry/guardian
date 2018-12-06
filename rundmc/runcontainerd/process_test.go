package runcontainerd_test

import (
	"errors"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Containerd Process", func() {
	var (
		logger         lager.Logger
		processManager *runcontainerdfakes.FakeProcessManager
		process        *runcontainerd.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-logger")
		processManager = new(runcontainerdfakes.FakeProcessManager)
		process = runcontainerd.NewProcess(logger, "container-id", "process-id", processManager)
	})

	Describe("ID", func() {
		It("returns the container ID", func() {
			Expect(process.ID()).To(Equal("container-id"))
		})
	})

	Describe("Wait", func() {
		var (
			exitCode int
			err      error
		)

		BeforeEach(func() {
			processManager.WaitReturns(42, nil)
		})

		JustBeforeEach(func() {
			exitCode, err = process.Wait()
		})

		It("returns the process exit code", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(42))
		})

		It("waits for the process", func() {
			Expect(processManager.WaitCallCount()).To(Equal(1))

			_, actualContainerID, actualProcessID := processManager.WaitArgsForCall(0)
			Expect(actualContainerID).To(Equal("container-id"))
			Expect(actualProcessID).To(Equal("process-id"))
		})

		Context("when waiting for the proccess fails", func() {
			BeforeEach(func() {
				processManager.WaitReturns(-1, errors.New("FAIL"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("FAIL"))
			})
		})
	})

	Describe("Signal", func() {
		Context("with a garden.SignalKill", func() {
			It("sends a SIGKILL to the process", func() {
				err := process.Signal(garden.SignalKill)
				Expect(err).NotTo(HaveOccurred())
				Expect(processManager.SignalCallCount()).To(Equal(1))

				_, actualContainerID, actualProcessID, actualSignal := processManager.SignalArgsForCall(0)
				Expect(actualContainerID).To(Equal("container-id"))
				Expect(actualProcessID).To(Equal("process-id"))
				Expect(actualSignal).To(Equal(syscall.SIGKILL))
			})
		})

		Context("with a garden.SignalKill", func() {
			It("sends a SIGKILL to the process", func() {
				err := process.Signal(garden.SignalTerminate)
				Expect(err).NotTo(HaveOccurred())
				Expect(processManager.SignalCallCount()).To(Equal(1))

				_, actualContainerID, actualProcessID, actualSignal := processManager.SignalArgsForCall(0)
				Expect(actualContainerID).To(Equal("container-id"))
				Expect(actualProcessID).To(Equal("process-id"))
				Expect(actualSignal).To(Equal(syscall.SIGTERM))
			})
		})

		Context("with an unknown signal", func() {
			It("returns an error", func() {
				err := process.Signal(-1)
				Expect(err).To(MatchError("Cannot convert garden signal -1 to syscall.Signal"))
			})
		})

		Context("when sending the signal to the process fails", func() {
			BeforeEach(func() {
				processManager.SignalReturns(errors.New("FAIL"))
			})

			It("returns the error", func() {
				err := process.Signal(garden.SignalTerminate)
				Expect(err).To(MatchError("FAIL"))
			})
		})
	})
})

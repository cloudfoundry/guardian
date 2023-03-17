package runcontainerd_test

import (
	"errors"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Containerd Process", func() {
	var (
		logger         *lagertest.TestLogger
		backingProcess *runcontainerdfakes.FakeBackingProcess
		process        *runcontainerd.Process
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-logger")
		backingProcess = new(runcontainerdfakes.FakeBackingProcess)
		backingProcess.IDReturns("process-id")
		process = runcontainerd.NewProcess(logger, backingProcess, false)
	})

	Describe("ID", func() {
		It("returns the process ID", func() {
			Expect(process.ID()).To(Equal("process-id"))
		})
	})

	Describe("Wait", func() {
		var (
			exitCode int
			err      error
		)

		BeforeEach(func() {
			backingProcess.WaitReturns(42, nil)
		})

		JustBeforeEach(func() {
			exitCode, err = process.Wait()
		})

		It("waits for the process exit code", func() {
			Expect(backingProcess.WaitCallCount()).To(Equal(1))
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCode).To(Equal(42))
		})

		Context("when waiting for the proccess fails", func() {
			BeforeEach(func() {
				backingProcess.WaitReturns(-1, errors.New("FAIL"))
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("FAIL"))
			})
		})

		Context("when CleanupProcessDirsOnWait=true", func() {
			BeforeEach(func() {
				process = runcontainerd.NewProcess(logger, backingProcess, true)
			})

			It("calls delete on the backing process", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(backingProcess.DeleteCallCount()).To(Equal(1))
			})

			Context("when deleting the backing process fails", func() {
				BeforeEach(func() {
					backingProcess.DeleteReturns(errors.New("oops"))
				})

				It("logs the error", func() {
					Expect(logger.LogMessages()).To(ContainElement(ContainSubstring("cleanup-failed-deleting-process")))
				})

				It("does not return the error", func() {
					Expect(err).NotTo(HaveOccurred())
				})
			})

		})
	})

	Describe("Signal", func() {
		Context("with a garden.SignalKill", func() {
			It("sends a SIGKILL to the process", func() {
				err := process.Signal(garden.SignalKill)
				Expect(err).NotTo(HaveOccurred())
				Expect(backingProcess.SignalCallCount()).To(Equal(1))
				Expect(backingProcess.SignalArgsForCall(0)).To(Equal(syscall.SIGKILL))
			})
		})

		Context("with a garden.SignalTerminate", func() {
			It("sends a SIGTERM to the process", func() {
				err := process.Signal(garden.SignalTerminate)
				Expect(err).NotTo(HaveOccurred())
				Expect(backingProcess.SignalCallCount()).To(Equal(1))
				Expect(backingProcess.SignalArgsForCall(0)).To(Equal(syscall.SIGTERM))
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
				backingProcess.SignalReturns(errors.New("FAIL"))
			})

			It("returns the error", func() {
				err := process.Signal(garden.SignalTerminate)
				Expect(err).To(MatchError("FAIL"))
			})
		})
	})
})

package group_runner_test

import (
	"errors"
	. "github.com/cloudfoundry/gunk/group_runner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/fake_runner"
	"os"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GroupRunner", func() {
	var (
		started chan struct{}

		groupRunner  ifrit.Runner
		groupProcess ifrit.Process

		childRunner1Errors chan error
		childRunner2Errors chan error
		childRunner3Errors chan error

		childRunner1 *fake_runner.FakeRunner
		childRunner2 *fake_runner.FakeRunner
		childRunner3 *fake_runner.FakeRunner

		Δ time.Duration = 10 * time.Millisecond
	)

	BeforeEach(func() {
		childRunner1Errors = make(chan error)
		childRunner2Errors = make(chan error)
		childRunner3Errors = make(chan error)

		childRunner1 = &fake_runner.FakeRunner{
			RunStub: func(signals <-chan os.Signal, ready chan<- struct{}) error {
				return <-childRunner1Errors
			},
		}

		childRunner2 = &fake_runner.FakeRunner{
			RunStub: func(signals <-chan os.Signal, ready chan<- struct{}) error {
				return <-childRunner2Errors
			},
		}

		childRunner3 = &fake_runner.FakeRunner{
			RunStub: func(signals <-chan os.Signal, ready chan<- struct{}) error {
				return <-childRunner3Errors
			},
		}

		groupRunner = New([]Member{
			{"child1", childRunner1},
			{"child2", childRunner2},
			{"child3", childRunner3},
		})

		started = make(chan struct{})
		go func() {
			groupProcess = ifrit.Envoke(groupRunner)
			close(started)
		}()
	})

	AfterEach(func() {
		close(childRunner1Errors)
		close(childRunner2Errors)
		close(childRunner3Errors)

		Eventually(started).Should(BeClosed())
		groupProcess.Signal(os.Kill)
		Eventually(groupProcess.Wait()).Should(Receive())
	})

	It("runs the first runner, then the second, then becomes ready", func() {
		Eventually(childRunner1.RunCallCount).Should(Equal(1))
		Consistently(childRunner2.RunCallCount, Δ).Should(BeZero())
		Consistently(started, Δ).ShouldNot(BeClosed())
		_, ready := childRunner1.RunArgsForCall(0)
		close(ready)

		Eventually(childRunner2.RunCallCount).Should(Equal(1))
		Consistently(childRunner3.RunCallCount, Δ).Should(BeZero())
		Consistently(started, Δ).ShouldNot(BeClosed())
		_, ready = childRunner2.RunArgsForCall(0)
		close(ready)

		Eventually(childRunner3.RunCallCount).Should(Equal(1))
		Consistently(started, Δ).ShouldNot(BeClosed())
		_, ready = childRunner3.RunArgsForCall(0)
		close(ready)

		Eventually(started).Should(BeClosed())
	})

	Describe("when all the runners are ready", func() {
		var (
			signal1 <-chan os.Signal
			signal2 <-chan os.Signal
			signal3 <-chan os.Signal
		)

		BeforeEach(func() {
			var ready chan<- struct{}

			Eventually(childRunner1.RunCallCount).Should(Equal(1))
			signal1, ready = childRunner1.RunArgsForCall(0)
			close(ready)

			Eventually(childRunner2.RunCallCount).Should(Equal(1))
			signal2, ready = childRunner2.RunArgsForCall(0)
			close(ready)

			Eventually(childRunner3.RunCallCount).Should(Equal(1))
			signal3, ready = childRunner3.RunArgsForCall(0)
			close(ready)

			<-started
		})

		Describe("when it receives a signal", func() {
			BeforeEach(func() {
				groupProcess.Signal(syscall.SIGUSR2)
			})

			It("sends the signal to all child runners", func() {
				Eventually(signal1).Should(Receive(Equal(syscall.SIGUSR2)))
				Eventually(signal2).Should(Receive(Equal(syscall.SIGUSR2)))
				Eventually(signal3).Should(Receive(Equal(syscall.SIGUSR2)))
			})

			It("doesn't send any more signals to remaining child processes", func() {
				Eventually(signal3).Should(Receive(Equal(syscall.SIGUSR2)))
				childRunner2Errors <- nil
				Consistently(signal3).ShouldNot(Receive())
			})
		})

		Describe("when a process exits cleanly", func() {
			BeforeEach(func() {
				childRunner1Errors <- nil
			})

			It("sends an interrupt signal to the other processes", func() {
				Eventually(signal2).Should(Receive(Equal(os.Interrupt)))
				Eventually(signal3).Should(Receive(Equal(os.Interrupt)))
			})

			It("does not exit", func() {
				Consistently(groupProcess.Wait(), Δ).ShouldNot(Receive())
			})

			Describe("when another process exits", func() {
				BeforeEach(func() {
					Eventually(signal3).Should(Receive(Equal(os.Interrupt)))
					childRunner2Errors <- nil
				})

				It("doesn't send any more signals to remaining child processes", func() {
					Consistently(signal3).ShouldNot(Receive())
				})
			})

			Describe("when all of the processes have exited cleanly", func() {
				BeforeEach(func() {
					childRunner2Errors <- nil
					childRunner3Errors <- nil
				})

				It("exits cleanly", func() {
					Eventually(groupProcess.Wait()).Should(Receive(BeNil()))
				})
			})

			Describe("when one of the processes exits with an error", func() {
				BeforeEach(func() {
					childRunner2Errors <- errors.New("Fail")
					childRunner3Errors <- nil
				})

				It("returns an error indicating which child processes failed", func() {
					var err error
					Eventually(groupProcess.Wait()).Should(Receive(&err))
					Expect(err).To(Equal(ExitTrace{
						{Member{"child1", childRunner1}, nil},
						{Member{"child2", childRunner2}, errors.New("Fail")},
						{Member{"child3", childRunner3}, nil},
					}))

				})
			})
		})
	})
})

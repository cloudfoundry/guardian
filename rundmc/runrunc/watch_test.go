package runrunc_test

import (
	"errors"
	"io"
	"os/exec"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
)

var _ = Describe("Watching for Events", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		runner *runrunc.OomWatcher
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		runner = runrunc.NewOomWatcher(commandRunner, runcBinary)

		runcBinary.EventsCommandStub = func(handle string) *exec.Cmd {
			return exec.Command("funC-events", "events", handle)
		}
	})

	It("blows up if `runc events` returns an error", func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "funC-events",
		}, func(cmd *exec.Cmd) error {
			return errors.New("boom")
		})

		Expect(runner.WatchEvents(logger, "some-container", nil)).To(MatchError("start: boom"))
	})

	Context("when runc events succeeds", func() {
		var (
			eventsCh chan string

			eventsNotifier *fakes.FakeEventsNotifier
		)

		BeforeEach(func() {
			eventsCh = make(chan string, 2)

			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-events",
			}, func(cmd *exec.Cmd) error {
				go func(eventsCh chan string, stdoutW io.WriteCloser) {
					defer stdoutW.Close()

					for eventJSON := range eventsCh {
						stdoutW.Write([]byte(eventJSON))
					}
				}(eventsCh, cmd.Stdout.(io.WriteCloser))

				return nil
			})

			eventsNotifier = new(fakes.FakeEventsNotifier)
		})

		It("reports an event if one happens", func() {
			defer close(eventsCh)

			waitCh := make(chan struct{})
			defer close(waitCh)
			commandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
				Path: "funC-events",
			}, func(cmd *exec.Cmd) error {
				<-waitCh
				return nil
			})

			go runner.WatchEvents(logger, "some-container", eventsNotifier)

			Consistently(eventsNotifier.OnEventCallCount).Should(Equal(0))

			eventsCh <- `{"type":"oom"}`
			Eventually(eventsNotifier.OnEventCallCount).Should(Equal(1))
			handle, event := eventsNotifier.OnEventArgsForCall(0)
			Expect(handle).To(Equal("some-container"))
			Expect(event).To(Equal("Out of memory"))

			eventsCh <- `{"type":"oom"}`
			Eventually(eventsNotifier.OnEventCallCount).Should(Equal(2))
			handle, event = eventsNotifier.OnEventArgsForCall(1)
			Expect(handle).To(Equal("some-container"))
			Expect(event).To(Equal("Out of memory"))
		})

		It("does not report non-OOM events", func() {
			defer close(eventsCh)

			go runner.WatchEvents(logger, "some-container", eventsNotifier)

			eventsCh <- `{"type":"stats"}`
			Consistently(eventsNotifier.OnEventCallCount).Should(Equal(0))
		})

		It("waits on the process to avoid zombies", func() {
			close(eventsCh)

			Expect(runner.WatchEvents(logger, "some-container", eventsNotifier)).To(Succeed())
			Eventually(commandRunner.WaitedCommands).Should(HaveLen(1))
			Expect(commandRunner.WaitedCommands()[0].Path).To(Equal("funC-events"))
		})
	})
})

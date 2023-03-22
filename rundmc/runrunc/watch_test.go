package runrunc_test

import (
	"errors"
	"io"
	"os/exec"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/guardian/rundmc/event"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
)

var _ = Describe("Watching for Events", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		oomWatcher *runrunc.OomWatcher
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		oomWatcher = runrunc.NewOomWatcher(commandRunner, runcBinary)

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

		Expect(oomWatcher.WatchEvents(logger, "some-container")).To(MatchError("start: boom"))
	})

	Context("when runc events succeeds", func() {
		var (
			eventsInputCh chan string
			oomEventsCh   <-chan event.Event
		)

		BeforeEach(func() {
			eventsInputCh = make(chan string, 2)

			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-events",
			}, func(cmd *exec.Cmd) error {
				go func(eventsCh chan string, stdoutW io.WriteCloser) {
					defer stdoutW.Close()

					for eventJSON := range eventsCh {
						stdoutW.Write([]byte(eventJSON))
					}
				}(eventsInputCh, cmd.Stdout.(io.WriteCloser))

				return nil
			})

			var err error
			oomEventsCh, err = oomWatcher.Events(logger)
			Expect(err).NotTo(HaveOccurred())
		})

		It("reports an event if one happens", func() {
			defer close(eventsInputCh)

			waitCh := make(chan struct{})
			defer close(waitCh)
			commandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
				Path: "funC-events",
			}, func(cmd *exec.Cmd) error {
				<-waitCh
				return nil
			})

			go oomWatcher.WatchEvents(logger, "some-container")

			Consistently(oomEventsCh).Should(BeEmpty())

			eventsInputCh <- `{"type":"oom"}`
			oomEvent := <-oomEventsCh
			Expect(oomEvent.ContainerID).To(Equal("some-container"))
			Expect(oomEvent.Message).To(Equal("Out of memory"))

			eventsInputCh <- `{"type":"oom"}`
			oomEvent = <-oomEventsCh
			Expect(oomEvent.ContainerID).To(Equal("some-container"))
			Expect(oomEvent.Message).To(Equal("Out of memory"))
		})

		It("does not report non-OOM events", func() {
			defer close(eventsInputCh)

			go oomWatcher.WatchEvents(logger, "some-container")

			eventsInputCh <- `{"type":"stats"}`
			Consistently(oomEventsCh).Should(BeEmpty())
		})

		It("waits on the process to avoid zombies", func() {
			close(eventsInputCh)

			Expect(oomWatcher.WatchEvents(logger, "some-container")).To(Succeed())
			Eventually(commandRunner.WaitedCommands).Should(HaveLen(1))
			Expect(commandRunner.WaitedCommands()[0].Path).To(Equal("funC-events"))
		})
	})
})

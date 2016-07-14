package runrunc_test

import (
	"errors"
	"os/exec"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
)

var _ = Describe("State", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runner        *fakes.FakeRuncCmdRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		stateCmdOutput string
		stateCmdExit   error

		stater *runrunc.Stater
	)

	BeforeEach(func() {
		runner = new(fakes.FakeRuncCmdRunner)
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		stater = runrunc.NewStater(runner, runcBinary)

		runcBinary.StateCommandStub = func(id, logFile string) *exec.Cmd {
			return exec.Command("funC-state", "--log", logFile, "state", id)
		}

		stateCmdExit = nil
		stateCmdOutput = `{
					"Pid": 4,
					"Status": "quite-a-status"
				}`
	})

	JustBeforeEach(func() {
		runner.RunAndLogStub = func(_ lager.Logger, fn runrunc.LoggingCmd) error {
			return commandRunner.Run(fn("potato.log"))
		}

		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "funC-state",
		}, func(cmd *exec.Cmd) error {
			cmd.Stdout.Write([]byte(stateCmdOutput))
			return stateCmdExit
		})
	})

	It("gets the bundle state", func() {
		state, err := stater.State(logger, "some-container")
		Expect(err).NotTo(HaveOccurred())

		Expect(state.Pid).To(Equal(4))
		Expect(state.Status).To(BeEquivalentTo("quite-a-status"))
	})

	It("forwards runc logs", func() {
		_, err := stater.State(logger, "some-container")
		Expect(err).NotTo(HaveOccurred())

		Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "funC-state",
			Args: []string{"--log", "potato.log", "state", "some-container"},
		}))

	})

	Context("when getting state fails", func() {
		BeforeEach(func() {
			stateCmdExit = errors.New("boom")
		})

		It("returns the error", func() {
			_, err := stater.State(logger, "some-container")
			Expect(err).To(
				MatchError(ContainSubstring("boom")),
			)
		})
	})

	Context("when the state output is not JSON", func() {
		BeforeEach(func() {
			stateCmdOutput = "potato"
		})

		It("returns a reasonable error", func() {
			_, err := stater.State(logger, "some-container")
			Expect(err).To(
				MatchError(ContainSubstring("runc state: invalid character 'p'")),
			)
		})
	})
})

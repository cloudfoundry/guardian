package runrunc_test

import (
	"errors"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
)

var _ = Describe("State", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		stateCmdOutput string
		stateCmdExit   error

		runner *runrunc.Stater
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		runner = runrunc.NewStater(commandRunner, runcBinary)

		runcBinary.StateCommandStub = func(id string) *exec.Cmd {
			return exec.Command("funC", "state", id)
		}

		stateCmdExit = nil
		stateCmdOutput = `{
					"Pid": 4,
					"Status": "quite-a-status"
				}`
	})

	JustBeforeEach(func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "funC",
			Args: []string{"state", "some-container"},
		}, func(cmd *exec.Cmd) error {
			cmd.Stdout.Write([]byte(stateCmdOutput))
			return stateCmdExit
		})
	})

	It("gets the bundle state", func() {
		state, err := runner.State(logger, "some-container")
		Expect(err).NotTo(HaveOccurred())

		Expect(state.Pid).To(Equal(4))
		Expect(state.Status).To(BeEquivalentTo("quite-a-status"))
	})

	Context("when getting state fails", func() {
		BeforeEach(func() {
			stateCmdExit = errors.New("boom")
		})

		It("returns the error", func() {
			_, err := runner.State(logger, "some-container")
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
			_, err := runner.State(logger, "some-container")
			Expect(err).To(
				MatchError(ContainSubstring("runc state: invalid character 'p'")),
			)
		})
	})
})

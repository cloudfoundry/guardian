package runrunc_test

import (
	"errors"
	"os/exec"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runner        *fakes.FakeRuncCmdRunner
		runcBinary    *fakes.FakeRuncBinary
		stater        *fakes.FakeRuncStater
		logger        *lagertest.TestLogger

		deleter *runrunc.Deleter
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		runner = new(fakes.FakeRuncCmdRunner)
		logger = lagertest.NewTestLogger("test")

		stater = new(fakes.FakeRuncStater)
		stater.StateReturns(runrunc.State{Status: runrunc.CreatedStatus}, nil)

	})

	JustBeforeEach(func() {
		deleter = runrunc.NewDeleter(runner, runcBinary, stater)

		runcBinary.DeleteCommandStub = func(id string, force bool, logFile string) *exec.Cmd {
			if force {
				return exec.Command("funC", "--log", logFile, "delete", id, "--force")
			} else {
				return exec.Command("funC", "--log", logFile, "delete", id)
			}
		}

		runner.RunAndLogStub = func(_ lager.Logger, fn runrunc.LoggingCmd) error {
			return commandRunner.Run(fn("potato.log"))
		}
	})

	It("runs 'runc delete' in the container directory using the logging runner", func() {
		Expect(deleter.Delete(logger, "some-container")).To(Succeed())
		Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "funC",
			Args: []string{"--log", "potato.log", "delete", "some-container"},
		}))
	})

	Context("when getting the state fails", func() {
		BeforeEach(func() {
			stater.StateReturns(runrunc.State{}, errors.New("poptato"))
		})

		It("propagates the error", func() {
			Expect(deleter.Delete(logger, "some-container")).To(Succeed())
			Expect(runner.RunAndLogCallCount()).To(Equal(0))
		})
	})

	Context("when state is running", func() {
		BeforeEach(func() {
			stater.StateReturns(runrunc.State{Status: runrunc.RunningStatus}, nil)
		})

		It("calls the delete command with force", func() {
			Expect(deleter.Delete(logger, "some-container")).To(Succeed())
			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "funC",
				Args: []string{"--log", "potato.log", "delete", "some-container", "--force"},
			}))
		})
	})

	Context("when state is created", func() {
		BeforeEach(func() {
			stater.StateReturns(runrunc.State{Status: runrunc.CreatedStatus}, nil)
		})

		It("calls the delete command", func() {
			Expect(deleter.Delete(logger, "some-container")).To(Succeed())
			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "funC",
				Args: []string{"--log", "potato.log", "delete", "some-container"},
			}))
		})
	})

	Context("when state is stopped", func() {
		BeforeEach(func() {
			stater.StateReturns(runrunc.State{Status: runrunc.StoppedStatus}, nil)
		})

		It("calls the delete command", func() {
			Expect(deleter.Delete(logger, "some-container")).To(Succeed())
			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "funC",
				Args: []string{"--log", "potato.log", "delete", "some-container"},
			}))
		})
	})

	Context("when state is such that it should not result in a delete", func() {
		BeforeEach(func() {
			stater.StateReturns(runrunc.State{Status: "random-state"}, nil)
		})

		It("calls the delete command with force", func() {
			Expect(deleter.Delete(logger, "some-container")).To(Succeed())
			Expect(runner.RunAndLogCallCount()).To(Equal(0))
		})
	})
})

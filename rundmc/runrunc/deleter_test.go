package runrunc_test

import (
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
		logger        *lagertest.TestLogger
		force         bool

		deleter *runrunc.Deleter
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		runner = new(fakes.FakeRuncCmdRunner)
		force = false
		logger = lagertest.NewTestLogger("test")

		deleter = runrunc.NewDeleter(runner, runcBinary)

		runcBinary.DeleteCommandStub = func(id string, force bool, logFile string) *exec.Cmd {
			return exec.Command("funC", "--log", logFile, "delete", id)
		}

		runner.RunAndLogStub = func(_ lager.Logger, fn runrunc.LoggingCmd) error {
			return commandRunner.Run(fn("potato.log"))
		}
	})

	It("runs 'runc delete' in the container directory using the logging runner", func() {
		Expect(deleter.Delete(logger, force, "some-container")).To(Succeed())
		Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "funC",
			Args: []string{"--log", "potato.log", "delete", "some-container"},
		}))
	})

	Context("when forced", func() {
		BeforeEach(func() {
			force = true
		})

		It("passes the force to the delete command", func() {
			Expect(deleter.Delete(logger, force, "some-container")).To(Succeed())
			Expect(runcBinary.DeleteCommandCallCount()).To(Equal(1))
			_, passedForce, _ := runcBinary.DeleteCommandArgsForCall(0)
			Expect(passedForce).To(Equal(force))
		})
	})
})

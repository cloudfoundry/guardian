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

var _ = Describe("RuncDeleter", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runner        *fakes.FakeRuncCmdRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		deleter *runrunc.RuncDeleter
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		runner = new(fakes.FakeRuncCmdRunner)
		logger = lagertest.NewTestLogger("test")

	})

	JustBeforeEach(func() {
		deleter = runrunc.NewDeleter(runner, runcBinary)

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
		Expect(deleter.Delete(logger, "some-container", false)).To(Succeed())
		Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "funC",
			Args: []string{"--log", "potato.log", "delete", "some-container"},
		}))
	})

	Context("when delete with force", func() {
		It("calls the delete command with force", func() {
			Expect(deleter.Delete(logger, "some-container", true)).To(Succeed())
			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "funC",
				Args: []string{"--log", "potato.log", "delete", "some-container", "--force"},
			}))
		})
	})
})

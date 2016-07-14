package runrunc_test

import (
	"os/exec"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delete", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runner        *fakes.FakeRuncCmdRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		deleter *runrunc.Deleter
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		runner = new(fakes.FakeRuncCmdRunner)
		logger = lagertest.NewTestLogger("test")

		deleter = runrunc.NewDeleter(runner, runcBinary)

		runcBinary.DeleteCommandStub = func(id, logFile string) *exec.Cmd {
			return exec.Command("funC", "--log", logFile, "delete", id)
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

})

package runrunc_test

import (
	"errors"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/fakes"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Kill", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		runner *runrunc.Killer
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		runner = runrunc.NewKiller(commandRunner, runcBinary)

		runcBinary.KillCommandStub = func(id, signal string) *exec.Cmd {
			return exec.Command("funC", "kill", id, signal)
		}
	})

	It("runs 'runc kill' in the container directory", func() {
		Expect(runner.Kill(logger, "some-container")).To(Succeed())
		Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "funC",
			Args: []string{"kill", "some-container", "KILL"},
		}))
	})

	It("returns any stderr output when 'runc kill' fails", func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{}, func(cmd *exec.Cmd) error {
			cmd.Stderr.Write([]byte("some error"))
			return errors.New("exit status banana")
		})

		Expect(runner.Kill(logger, "some-container")).To(MatchError("runc kill: exit status banana: some error"))
	})
})

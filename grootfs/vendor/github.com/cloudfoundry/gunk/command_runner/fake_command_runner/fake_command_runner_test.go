package fake_command_runner_test

import (
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"

	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FakeCommandRunner", func() {

	var (
		runner    *fake_command_runner.FakeCommandRunner
		cmd, cmd2 *exec.Cmd
	)

	BeforeEach(func() {
		runner = fake_command_runner.New()
		cmd = &exec.Cmd{}
		cmd2 = &exec.Cmd{}
	})

	Describe("Kill", func() {
		It("should record Kill commands", func() {
			runner.Kill(cmd)
			Expect(runner.KilledCommands()).To(Equal([]*exec.Cmd{cmd}))
		})

		// This may seem like an odd test, but it exposed a bug.
		It("should not confuse Kill and Wait", func() {
			runner.Kill(cmd)
			runner.Wait(cmd2)
			Expect(runner.KilledCommands()).To(Equal([]*exec.Cmd{cmd}))
		})
	})

	Describe("Wait", func() {
		It("should record Wait commands", func() {
			runner.Wait(cmd)
			Expect(runner.WaitedCommands()).To(Equal([]*exec.Cmd{cmd}))
		})

		It("should not confuse Wait and Kill", func() {
			runner.Wait(cmd)
			runner.Kill(cmd2)
			Expect(runner.WaitedCommands()).To(Equal([]*exec.Cmd{cmd}))
		})
	})

})

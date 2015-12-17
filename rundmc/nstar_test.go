package rundmc_test

import (
	"errors"
	"io"
	"os/exec"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Nstar", func() {
	var (
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		someStream        io.Reader

		nstar rundmc.NstarRunner
	)

	BeforeEach(func() {
		fakeCommandRunner = fake_command_runner.New()
		nstar = rundmc.NewNstarRunner(
			"path-to-nstar",
			"path-to-tar",
			fakeCommandRunner,
		)

		someStream = gbytes.NewBuffer()
	})

	Context("when it executes succesfully", func() {
		BeforeEach(func() {
			Expect(nstar.StreamIn(lagertest.NewTestLogger("test"), 12, "some-path", "some-user", someStream)).To(Succeed())
		})

		It("executes the nstar command with the right arguments", func() {
			Expect(fakeCommandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "path-to-nstar",
				Args: []string{
					"path-to-tar",
					"12",
					"some-user",
					"some-path",
				},
			}))
		})

		It("attaches the tarStream reader to stdin", func() {
			Expect(fakeCommandRunner.ExecutedCommands()[0].Stdin).To(Equal(someStream))
		})
	})

	Context("when it fails", func() {
		It("returns the contents of stdout and error on failure", func() {
			fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{}, func(cmd *exec.Cmd) error {
				cmd.Stderr.Write([]byte("some error output"))
				cmd.Stdout.Write([]byte("some std output"))
				return errors.New("someerror")
			})

			Expect(nstar.StreamIn(lagertest.NewTestLogger("test"), 12, "some-path", "some-user", someStream)).To(
				MatchError(ContainSubstring("some error output")),
			)

			Expect(nstar.StreamIn(lagertest.NewTestLogger("test"), 12, "some-path", "some-user", someStream)).To(
				MatchError(ContainSubstring("some std output")),
			)
		})
	})
})

package rundmc_test

import (
	"errors"
	"io"
	"io/ioutil"
	"os/exec"

	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Nstar", func() {
	var (
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
		nstar             rundmc.NstarRunner
	)

	BeforeEach(func() {
		fakeCommandRunner = fake_command_runner.New()
		nstar = rundmc.NewNstarRunner(
			"path-to-nstar",
			"path-to-tar",
			fakeCommandRunner,
		)
	})

	Describe("StreamIn", func() {
		var someStream io.Reader

		BeforeEach(func() {
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

		Context("when no user specified", func() {
			It("streams the input to tar as root", func() {
				buffer := gbytes.NewBuffer()
				buffer.Write([]byte("the-tar-content"))

				fakeCommandRunner.WhenRunning(
					fake_command_runner.CommandSpec{},
					func(cmd *exec.Cmd) error {
						bytes, err := ioutil.ReadAll(cmd.Stdin)
						Expect(err).ToNot(HaveOccurred())

						Expect(string(bytes)).To(Equal("the-tar-content"))

						return nil
					},
				)

				Expect(nstar.StreamIn(lagertest.NewTestLogger("test"), 12, "some-path", "", buffer)).To(Succeed())
				Expect(fakeCommandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
					Path: "path-to-nstar",
					Args: []string{
						"path-to-tar",
						"12",
						"root",
						"some-path",
					},
				},
				))
			})
		})
	})

	Describe("StreamOut", func() {
		It("streams the output to the destination as the specified user", func() {
			fakeCommandRunner.WhenRunning(
				fake_command_runner.CommandSpec{},
				func(cmd *exec.Cmd) error {
					_, err := cmd.Stdout.Write([]byte("the-compressed-content"))
					Expect(err).ToNot(HaveOccurred())
					return nil
				},
			)

			reader, err := nstar.StreamOut(lagertest.NewTestLogger("test"), 12, "some-dir/some-file", "some-user")
			Expect(err).ToNot(HaveOccurred())

			bytes, err := ioutil.ReadAll(reader)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(bytes)).To(Equal("the-compressed-content"))

			Expect(fakeCommandRunner).To(HaveBackgrounded(fake_command_runner.CommandSpec{
				Path: "path-to-nstar",
				Args: []string{
					"path-to-tar",
					"12",
					"some-user",
					"some-dir",
					"some-file",
				},
			}))
		})

		Context("when there's a trailing slash", func() {
			It("compresses the directory's contents", func() {
				_, err := nstar.StreamOut(lagertest.NewTestLogger("test"), 12, "some-path/directory/dst/", "some-user")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeCommandRunner).To(HaveBackgrounded(
					fake_command_runner.CommandSpec{
						Path: "path-to-nstar",
						Args: []string{
							"path-to-tar",
							"12",
							"some-user",
							"some-path/directory/dst/",
							".",
						},
					},
				))
			})
		})

		It("closes the server-side dupe of of the pipe's write end", func() {
			var outPipe io.Writer

			fakeCommandRunner.WhenRunning(
				fake_command_runner.CommandSpec{},
				func(cmd *exec.Cmd) error {
					outPipe = cmd.Stdout
					return nil
				},
			)

			_, err := nstar.StreamOut(lagertest.NewTestLogger("test"), 12, "some-path", "some-user")
			Expect(err).ToNot(HaveOccurred())
			Expect(outPipe).ToNot(BeNil())

			_, err = outPipe.Write([]byte("sup"))
			Expect(err).To(HaveOccurred())
		})

		Context("when no user specified", func() {
			It("streams the output of tar as root", func() {
				fakeCommandRunner.WhenRunning(
					fake_command_runner.CommandSpec{},
					func(cmd *exec.Cmd) error {
						_, err := cmd.Stdout.Write([]byte("the-compressed-content"))
						Expect(err).ToNot(HaveOccurred())

						return nil
					},
				)

				reader, err := nstar.StreamOut(lagertest.NewTestLogger("test"), 12, "some-dir/some-file", "")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeCommandRunner).To(HaveBackgrounded(fake_command_runner.CommandSpec{
					Path: "path-to-nstar",
					Args: []string{
						"path-to-tar",
						"12",
						"root",
						"some-dir",
						"some-file",
					},
				},
				))

				bytes, err := ioutil.ReadAll(reader)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(bytes)).To(Equal("the-compressed-content"))

			})
		})
	})

	Context("when it fails", func() {
		It("returns the contents of stderr on failure", func() {
			fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{}, func(cmd *exec.Cmd) error {
				cmd.Stderr.Write([]byte("some error output"))
				return errors.New("someerror")
			})

			stream, err := nstar.StreamOut(lagertest.NewTestLogger("test"), 12, "some-path", "some-user")
			Expect(stream).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("some error output")))
		})
	})
})

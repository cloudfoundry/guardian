package netplugin_test

import (
	"errors"
	"os/exec"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/netplugin"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExternalBinaryNetworker", func() {
	var (
		containerSpec     garden.ContainerSpec
		fakeCommandRunner *fake_command_runner.FakeCommandRunner
	)

	BeforeEach(func() {
		fakeCommandRunner = fake_command_runner.New()
		containerSpec = garden.ContainerSpec{
			Handle:  "some-handle",
			Network: "potato",
			Properties: garden.Properties{
				"some-key":               "some-value",
				"some-other-key":         "some-other-value",
				"network.some-key":       "some-network-value",
				"network.some-other-key": "some-other-network-value",
			},
		}
	})

	Describe("Network", func() {
		It("executes the external plugin with the correct args", func() {
			plugin := netplugin.New(fakeCommandRunner, "some/path")
			err := plugin.Network(lagertest.NewTestLogger("test"), containerSpec, 42)
			Expect(err).NotTo(HaveOccurred())

			cmd := fakeCommandRunner.ExecutedCommands()[0]
			Expect(cmd.Path).To(Equal("some/path"))

			Expect(cmd.Args[:7]).To(Equal([]string{
				"some/path",
				"--action", "up",
				"--handle", "some-handle",
				"--network", "potato",
			}))

			Expect(cmd.Args[7]).To(Equal("--properties"))
			Expect(cmd.Args[8]).To(MatchJSON(`{
					"some-key":       "some-network-value",
					"some-other-key": "some-other-network-value"
			}`))
		})

		Context("when there are extra args", func() {
			It("prepends the extra args before the standard hook parameters", func() {
				plugin := netplugin.New(fakeCommandRunner, "some/path", "arg1", "arg2", "arg3")
				err := plugin.Network(lagertest.NewTestLogger("test"), containerSpec, 42)
				Expect(err).NotTo(HaveOccurred())

				cmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(cmd.Args[:6]).To(Equal([]string{
					"some/path",
					"arg1",
					"arg2",
					"arg3",
					"--action", "up",
				}))
			})
		})

		Context("when the external plugin errors", func() {
			It("returns the error", func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "some/path",
				}, func(cmd *exec.Cmd) error {
					return errors.New("boom")
				})

				plugin := netplugin.New(fakeCommandRunner, "some/path")
				Expect(plugin.Network(nil, containerSpec, 42)).To(MatchError("boom"))
			})
		})
	})

	Describe("Destroy", func() {
		It("executes the external plugin with the correct args", func() {
			plugin := netplugin.New(fakeCommandRunner, "some/path")
			Expect(plugin.Destroy(lagertest.NewTestLogger("test"), "my-handle")).To(Succeed())

			cmd := fakeCommandRunner.ExecutedCommands()[0]
			Expect(cmd.Path).To(Equal("some/path"))

			Expect(cmd.Args[:5]).To(Equal([]string{
				"some/path",
				"--action", "down",
				"--handle", "my-handle",
			}))
		})

		Context("when there are extra args", func() {
			It("prepends the extra args before the standard hook parameters", func() {
				plugin := netplugin.New(fakeCommandRunner, "some/path", "arg1", "arg2", "arg3")
				err := plugin.Destroy(lagertest.NewTestLogger("test"), "my-handle")
				Expect(err).NotTo(HaveOccurred())

				cmd := fakeCommandRunner.ExecutedCommands()[0]

				Expect(cmd.Args[:8]).To(Equal([]string{
					"some/path",
					"arg1",
					"arg2",
					"arg3",
					"--action", "down",
					"--handle", "my-handle",
				}))
			})
		})
		Context("when the external plugin errors", func() {
			It("returns the error", func() {
				fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
					Path: "some/path",
				}, func(cmd *exec.Cmd) error {
					return errors.New("boom")
				})

				plugin := netplugin.New(fakeCommandRunner, "some/path")
				Expect(plugin.Network(nil, containerSpec, 42)).To(MatchError("boom"))
			})
		})
	})
})

package netplugin_test

import (
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/netplugin"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Plugin", func() {
	Describe("Hooks", func() {
		var containerSpec garden.ContainerSpec

		BeforeEach(func() {
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

		It("returns a Hooks struct with the correct path", func() {
			plugin := netplugin.New("some/path")
			hooks, err := plugin.Hooks(lagertest.NewTestLogger("test"), containerSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(hooks.Prestart.Path).To(Equal("some/path"))
			Expect(hooks.Poststop.Path).To(Equal("some/path"))
		})

		It("uses the plugin name as the first argument", func() {
			plugin := netplugin.New("some/path")
			hooks, err := plugin.Hooks(lagertest.NewTestLogger("test"), containerSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(hooks.Prestart.Args[0]).To(Equal("some/path"))
			Expect(hooks.Poststop.Args[0]).To(Equal("some/path"))
		})

		It("returns a Hook struct with the correct args", func() {
			plugin := netplugin.New("some/path")
			hooks, err := plugin.Hooks(lagertest.NewTestLogger("test"), containerSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(hooks.Prestart.Args[:7]).To(Equal([]string{
				"some/path",
				"--action", "up",
				"--handle", "some-handle",
				"--network", "potato",
			}))

			Expect(hooks.Prestart.Args[7]).To(Equal("--properties"))
			Expect(hooks.Prestart.Args[8]).To(MatchJSON(`{
					"some-key":       "some-network-value",
					"some-other-key": "some-other-network-value"
			}`))

			Expect(hooks.Poststop.Args[:7]).To(Equal([]string{
				"some/path",
				"--action", "down",
				"--handle", "some-handle",
				"--network", "potato",
			}))

			Expect(hooks.Poststop.Args[7]).To(Equal("--properties"))
			Expect(hooks.Poststop.Args[8]).To(MatchJSON(`{
					"some-key":       "some-network-value",
					"some-other-key": "some-other-network-value"
			}`))
		})

		Context("when there are extra args", func() {
			It("prepends the extra args before the standard hook parameters", func() {
				plugin := netplugin.New("some/path", "arg1", "arg2", "arg3")
				hooks, err := plugin.Hooks(lagertest.NewTestLogger("test"), containerSpec)
				Expect(err).NotTo(HaveOccurred())

				Expect(hooks.Prestart.Args[:6]).To(Equal([]string{
					"some/path",
					"arg1",
					"arg2",
					"arg3",
					"--action", "up",
				}))
				Expect(hooks.Poststop.Args[:6]).To(Equal([]string{
					"some/path",
					"arg1",
					"arg2",
					"arg3",
					"--action", "down",
				}))
			})
		})
	})
})

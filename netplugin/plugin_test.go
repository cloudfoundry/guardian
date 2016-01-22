package netplugin_test

import (
	"github.com/cloudfoundry-incubator/guardian/netplugin"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Plugin", func() {
	Describe("Hook", func() {
		It("returns a Hook struct with the correct params", func() {
			plugin := netplugin.New("some/path")
			hook, err := plugin.Hook(lagertest.NewTestLogger("test"), "some-handle", "potato")
			Expect(err).NotTo(HaveOccurred())

			Expect(hook.Path).To(Equal("some/path"))
		})

		Context("when no extra args are passed", func() {
			It("uses the plugin name as the first argument", func() {
				plugin := netplugin.New("some/path")
				hook, err := plugin.Hook(lagertest.NewTestLogger("test"), "some-handle", "potato")
				Expect(err).NotTo(HaveOccurred())

				Expect(hook.Args[0]).To(Equal("some/path"))
			})

			It("returns a Hook struct with the correct args", func() {
				plugin := netplugin.New("some/path")

				hook, err := plugin.Hook(
					lagertest.NewTestLogger("test"),
					"some-handle",
					"potato",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(hook.Args).To(Equal([]string{"some/path", "up", "--handle", "some-handle", "--network", "potato"}))
			})
		})

		Context("when there are extra args", func() {
			It("prepends the extra args before the standard hook parameters", func() {
				plugin := netplugin.New("some/path", "arg1", "arg2")

				hook, err := plugin.Hook(
					lagertest.NewTestLogger("test"),
					"some-handle",
					"potato",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(hook.Args).To(Equal([]string{"some/path", "arg1", "arg2", "up", "--handle", "some-handle", "--network", "potato"}))
			})
		})
	})
})

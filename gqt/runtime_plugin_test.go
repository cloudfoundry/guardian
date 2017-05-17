package gqt_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runtime Plugin", func() {
	var (
		args   []string
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		args = []string{}
	})

	JustBeforeEach(func() {
		client = startGarden(args...)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Context("when a runtime plugin is provided", func() {
		BeforeEach(func() {
			args = append(args, "--runtime-plugin", binaries.RuntimePlugin)
		})

		Context("and a container is successfully created", func() {
			var handle = "some-handle"

			JustBeforeEach(func() {
				client.Create(garden.ContainerSpec{Handle: handle})
			})

			It("executes the plugin, passing the correct args", func() {
				pluginArgsBytes, err := ioutil.ReadFile(filepath.Join(client.Tmpdir, "args"))
				Expect(err).ToNot(HaveOccurred())

				pluginArgs := strings.Split(string(pluginArgsBytes), " ")
				Expect(pluginArgs).To(ConsistOf(
					binaries.RuntimePlugin,
					"--debug",
					"--log", HaveSuffix(filepath.Join("containers", handle, "create.log")),
					"--newuidmap", HaveSuffix("newuidmap"),
					"--newgidmap", HaveSuffix("newgidmap"),
					"create",
					"--no-new-keyring",
					"--bundle", HaveSuffix(filepath.Join("containers", handle)),
					"--pid-file", HaveSuffix(filepath.Join("containers", handle, "pidfile")),
					handle,
				))
			})
		})
	})
})

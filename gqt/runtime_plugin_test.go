package gqt_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
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
			args = append(
				args,
				"--runtime-plugin", binaries.RuntimePlugin,
				"--network-plugin", binaries.NoopPlugin,
			)
		})

		Describe("creating a container", func() {
			var (
				handle       = fmt.Sprintf("some-handle-%d", GinkgoParallelNode())
				container    garden.Container
				argsFilepath string
			)

			JustBeforeEach(func() {
				argsFilepath = filepath.Join(client.Tmpdir, "create-args")
				var err error
				container, err = client.Create(garden.ContainerSpec{Handle: handle})
				Expect(err).ToNot(HaveOccurred())
			})

			It("executes the plugin, passing the correct args for create", func() {
				Expect(readPluginArgs(argsFilepath)).To(ConsistOf(
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

			Describe("starting a process", func() {
				JustBeforeEach(func() {
					argsFilepath = filepath.Join(client.Tmpdir, "exec-args")

					_, err := container.Run(garden.ProcessSpec{Path: "some-idiosyncratic-binary"}, garden.ProcessIO{
						Stdout: GinkgoWriter,
						Stderr: GinkgoWriter,
					})
					Expect(err).ToNot(HaveOccurred())
				})

				It("executes the plugin, passing the correct args for exec", func() {
					Expect(readPluginArgs(argsFilepath)).To(ConsistOf(
						binaries.RuntimePlugin,
						"--debug",
						"--log", MatchRegexp(".*"),
						"exec",
						"-d",
						"-p", MatchRegexp(".*"),
						"--pid-file", MatchRegexp(".*"),
						handle,
					))
				})

				It("passes the spec serialised into a file", func() {
					processSpecFilePath := filepath.Join(client.Tmpdir, "exec-process-spec")
					Eventually(processSpecFilePath).Should(BeAnExistingFile())
					processSpecFile, err := os.Open(processSpecFilePath)
					Expect(err).ToNot(HaveOccurred())
					defer processSpecFile.Close()

					var processSpec runrunc.PreparedSpec
					Expect(json.NewDecoder(processSpecFile).Decode(&processSpec)).To(Succeed())
					Expect(processSpec.Process.Args[0]).To(Equal("some-idiosyncratic-binary"))
				})
			})
		})
	})
})

func readPluginArgs(argsFilePath string) []string {
	Eventually(argsFilePath).Should(BeAnExistingFile())
	pluginArgsBytes, err := ioutil.ReadFile(argsFilePath)
	Expect(err).ToNot(HaveOccurred())
	return strings.Split(string(pluginArgsBytes), " ")
}

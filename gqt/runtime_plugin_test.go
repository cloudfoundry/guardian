package gqt_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gqt/runner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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
				var (
					runtimePluginExitCode int
					stdoutContents        string
					stderrContents        string

					process      garden.Process
					stdoutWriter *gbytes.Buffer
					stderrWriter *gbytes.Buffer
					runErr       error
				)

				BeforeEach(func() {
					runtimePluginExitCode = 0

					stdoutWriter = gbytes.NewBuffer()
					stderrWriter = gbytes.NewBuffer()
				})

				JustBeforeEach(func() {
					argsFilepath = filepath.Join(client.Tmpdir, "exec-args")

					process, runErr = container.Run(garden.ProcessSpec{
						Path: "some-idiosyncratic-binary",
						Args: []string{fmt.Sprintf("%d", runtimePluginExitCode), stdoutContents, stderrContents},
					}, garden.ProcessIO{
						Stdout: io.MultiWriter(GinkgoWriter, stdoutWriter),
						Stderr: io.MultiWriter(GinkgoWriter, stderrWriter),
					})
				})

				It("returns no error", func() {
					Expect(runErr).NotTo(HaveOccurred())
				})

				It("executes the plugin, passing the correct args for exec", func() {
					pluginArgs := []interface{}{
						binaries.RuntimePlugin,
						"--debug",
						"--log", MatchRegexp(".*"),
						"exec",
						"-p", MatchRegexp(".*"),
						"--pid-file", MatchRegexp(".*"),
						handle,
					}
					if runtime.GOOS != "windows" {
						pluginArgs = append(pluginArgs, "-d")
					}
					Expect(readPluginArgs(argsFilepath)).To(ConsistOf(pluginArgs))
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

				Describe("runtime plugin stdio", func() {
					BeforeEach(func() {
						stdoutContents = "some stdout content"
						stderrContents = "some stderr content"
					})

					It("returns the runtime plugin's stdout", func() {
						Eventually(stdoutWriter).Should(gbytes.Say(stdoutContents))
					})

					It("returns the runtime plugin's stderr", func() {
						Eventually(stderrWriter).Should(gbytes.Say(stderrContents))
					})
				})

				Context("when the runtime plugin exits with 32", func() {
					BeforeEach(func() {
						runtimePluginExitCode = 32
					})

					It("returns an error because the runtime plugin exits non-zero on Linux", func() {
						onlyOnLinux()
						Expect(runErr).To(MatchError(ContainSubstring("exit status 32")))
					})

					It("returns no error on Windows", func() {
						onlyOnWindows()
						Expect(runErr).NotTo(HaveOccurred())
					})
				})

				Describe("the returned process", func() {
					Describe("Wait", func() {
						It("returns the exit status of the runtime plugin", func() {
							Expect(process.Wait()).To(Equal(0))
						})

						Context("when the runtime plugin exits with 42", func() {
							BeforeEach(func() {
								runtimePluginExitCode = 42
							})

							It("returns the exit status of the runtime plugin on Windows", func() {
								onlyOnWindows()
								Expect(process.Wait()).To(Equal(42))
							})
						})
					})
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

func onlyOnLinux() {
	onlyOn("linux")
}

func onlyOnWindows() {
	onlyOn("windows")
}

func onlyOn(goos string) {
	if runtime.GOOS != goos {
		Skip(goos + " only")
	}
}

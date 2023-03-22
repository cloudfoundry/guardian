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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gstruct"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Runtime Plugin", func() {
	var (
		client       *runner.RunningGarden
		handle       string
		argsFilepath string
	)

	BeforeEach(func() {
		skipIfContainerd()
		handle = fmt.Sprintf("runtime-plugin-test-handle-%s", config.Tag)
		config.RuntimePluginBin = binaries.RuntimePlugin
		config.NetworkPluginBin = binaries.NetworkPlugin
		config.ImagePluginBin = binaries.NoopPlugin
		config.RuntimePluginExtraArgs = []string{
			`"--image-store"`, `some-image-store`,
		}
		blkioWeight := uint64(200)
		config.DefaultBlkioWeight = &blkioWeight
	})

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
	})

	Describe("creating a container", func() {
		JustBeforeEach(func() {
			argsFilepath = filepath.Join(client.TmpDir, "run-args")
		})

		It("executes the plugin, passing the correct args for create", func() {
			_, err := client.Create(garden.ContainerSpec{Handle: handle})
			Expect(err).ToNot(HaveOccurred())

			Expect(readPluginArgs(argsFilepath)).To(ConsistOf(
				binaries.RuntimePlugin,
				"--debug",
				"--log", HaveSuffix(filepath.Join("containers", handle, "create.log")),
				"--log-format", "json",
				"--image-store", "some-image-store",
				"run",
				"--detach",
				"--no-new-keyring",
				"--bundle", HaveSuffix(filepath.Join("containers", handle)),
				"--pid-file", HaveSuffix(filepath.Join("containers", handle, "pidfile")),
				handle,
			))
		})

		Describe("limits", func() {
			var (
				containerSpec garden.ContainerSpec
				bundle        specs.Spec
			)

			BeforeEach(func() {
				containerSpec = garden.ContainerSpec{
					Handle: handle,
					Limits: garden.Limits{
						Memory: garden.MemoryLimits{
							LimitInBytes: 1 * 1024 * 1024,
						},
						CPU: garden.CPULimits{
							LimitInShares: 10,
						},
						Pid: garden.PidLimits{
							Max: 300,
						},
					},
				}
			})

			JustBeforeEach(func() {
				_, err := client.Create(containerSpec)
				Expect(err).ToNot(HaveOccurred())

				pluginArgs := readPluginArgs(argsFilepath)
				Expect(pluginArgs[11]).To(Equal("--bundle"))

				bundlePath := filepath.Join(pluginArgs[12], "config.json")
				bundleBytes, err := ioutil.ReadFile(bundlePath)
				Expect(err).NotTo(HaveOccurred())

				bundle = specs.Spec{}
				err = json.Unmarshal(bundleBytes, &bundle)
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("linux", func() {
				BeforeEach(func() {
					onlyOnLinux()
				})
				It("sets the memory limit", func() {
					Expect(bundle.Linux.Resources.Memory.Limit).To(PointTo(Equal(int64(1 * 1024 * 1024))))
				})

				It("sets the CPU shares", func() {
					Expect(bundle.Linux.Resources.CPU.Shares).To(PointTo(Equal(uint64(10))))
				})

				It("sets BlockIO", func() {
					Expect(bundle.Linux.Resources.BlockIO.Weight).To(PointTo(Equal(uint16(200))))
				})

				It("sets pid limits", func() {
					Expect(bundle.Linux.Resources.Pids.Limit).To(Equal(int64(300)))
				})
			})

			Describe("windows", func() {
				BeforeEach(func() {
					onlyOnWindows()
				})
				It("sets the memory limit", func() {
					Expect(bundle.Windows.Resources.Memory.Limit).To(PointTo(Equal(uint64(1 * 1024 * 1024))))
				})

				It("sets the CPU shares", func() {
					Expect(bundle.Windows.Resources.CPU.Shares).To(PointTo(Equal(uint16(10))))
				})

				When("CPU weight is specified", func() {
					BeforeEach(func() {
						containerSpec.Limits.CPU.Weight = 20
					})

					It("sets the CPU shares to the CPU weight", func() {
						Expect(bundle.Windows.Resources.CPU.Shares).To(PointTo(Equal(uint16(20))))
					})
				})
			})
		})
	})

	Describe("creating a pea", func() {
		JustBeforeEach(func() {
			argsFilepath = filepath.Join(client.TmpDir, "run-args")
		})

		// winc is currently not able to support run --detach for the pea usecase.
		// Additional context: https://www.pivotaltracker.com/story/show/155593711.
		Context("when run on Linux", func() {
			It("executes the plugin, passing the correct args for whatever a pea needs", func() {
				onlyOnLinux()

				container, err := client.Create(garden.ContainerSpec{Handle: handle})
				Expect(err).ToNot(HaveOccurred())

				process, err := container.Run(garden.ProcessSpec{
					Path:  "echo",
					Args:  []string{"hello"},
					Image: garden.ImageRef{URI: defaultTestRootFS},
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				procId := process.ID()
				Expect(readPluginArgs(argsFilepath)).To(ConsistOf(
					binaries.RuntimePlugin,
					"--root", getRuncRoot(),
					"--debug",
					"--log", MatchRegexp(`/proc/\d+/fd/4`),
					"--log-format", "json",
					"run",
					"--detach",
					"--pid-file", HaveSuffix(filepath.Join("containers", handle, "processes", procId, "pidfile")),
					"--no-new-keyring",
					"--bundle", HaveSuffix(filepath.Join("containers", handle, "processes", procId)),
					procId,
				))
			})
		})

		Context("when run on Windows", func() {
			It("executes the plugin, passing the correct args for whatever a pea needs", func() {
				onlyOnWindows()

				container, err := client.Create(garden.ContainerSpec{Handle: handle})
				Expect(err).ToNot(HaveOccurred())

				process, err := container.Run(garden.ProcessSpec{
					Path:  "cmd.exe",
					Args:  []string{"echo", "hello"},
					Image: garden.ImageRef{URI: defaultTestRootFS},
				}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				_, _ = process.Wait()

				procId := process.ID()
				Expect(readPluginArgs(argsFilepath)).To(ConsistOf(
					binaries.RuntimePlugin,
					"--debug",
					"--log-handle", MatchRegexp(`\d+`),
					"--log-format", "json",
					"run",
					"--pid-file", HaveSuffix(filepath.Join("containers", handle, "processes", procId, "pidfile")),
					"--bundle", HaveSuffix(filepath.Join("containers", handle, "processes", procId)),
					procId,
				))
			})
		})
	})

	Context("when the network plugin returns configuration", func() {
		BeforeEach(func() {
			pluginReturn := `{
					"properties":{
						"foo":"bar",
						"kawasaki.mtu":"1499",
						"garden.network.container-ip":"10.255.10.10",
						"garden.network.host-ip":"255.255.255.255"
					},
					"dns_servers": [
						"1.2.3.4",
						"1.2.3.5"
					]
			  }`
			config.NetworkPluginExtraArgs = []string{
				"--output", pluginReturn,
			}
		})

		It("succeeds", func() {
			_, err := client.Create(garden.ContainerSpec{Handle: handle})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when runtime plugin state returns a pid of zero for a created container", func() {
		var networkPluginArgsFile string

		BeforeEach(func() {
			networkPluginArgsFile = filepath.Join(config.TmpDir, fmt.Sprintf("network-a-%d", GinkgoParallelProcess()))
			config.NetworkPluginExtraArgs = []string{"--args-file", networkPluginArgsFile}
		})

		AfterEach(func() {
			Expect(os.Remove(networkPluginArgsFile)).To(Succeed())
		})

		JustBeforeEach(func() {
			Expect(ioutil.WriteFile(
				filepath.Join(client.TmpDir, "runtime-state-output"),
				[]byte(`{"pid": 0, "status": "created"}`),
				0600,
			)).To(Succeed())
		})

		It("returns an error without calling the network plugin", func() {
			_, err := client.Create(garden.ContainerSpec{Handle: handle})
			Expect(err).To(HaveOccurred())
			Expect(readFile(networkPluginArgsFile)).NotTo(ContainSubstring("up"))
		})
	})

	Describe("starting a process", func() {
		var (
			runtimePluginExitCode int
			stdoutContents        string
			stderrContents        string

			process garden.Process
			stdout  *gbytes.Buffer
			stderr  *gbytes.Buffer
			runErr  error
		)

		BeforeEach(func() {
			runtimePluginExitCode = 0

			stdout = gbytes.NewBuffer()
			stderr = gbytes.NewBuffer()
		})

		JustBeforeEach(func() {
			argsFilepath = filepath.Join(client.TmpDir, "exec-args")

			container, err := client.Create(garden.ContainerSpec{Handle: handle})
			Expect(err).ToNot(HaveOccurred())

			process, runErr = container.Run(garden.ProcessSpec{
				Path: "some-idiosyncratic-binary",
				Args: []string{fmt.Sprintf("%d", runtimePluginExitCode), stdoutContents, stderrContents},
			}, garden.ProcessIO{
				Stdout: io.MultiWriter(GinkgoWriter, stdout),
				Stderr: io.MultiWriter(GinkgoWriter, stderr),
			})
		})

		It("returns no error", func() {
			Expect(runErr).NotTo(HaveOccurred())
		})

		It("executes the plugin, passing the correct args for exec", func() {
			pluginArgs := []interface{}{
				binaries.RuntimePlugin,
				"--root", getRuncRoot(),
				"--debug",
				"--log", MatchRegexp(".*"),
				"--log-format", "json",
				"exec",
				"-p", MatchRegexp(".*"),
				"--pid-file", MatchRegexp(".*"),
				handle,
				"--detach",
			}

			if runtime.GOOS == "windows" {
				pluginArgs = []interface{}{
					binaries.RuntimePlugin,
					"--debug",
					"--log-handle", MatchRegexp(`\d`),
					"--log-format", "json",
					"exec",
					"-p", MatchRegexp(".*"),
					"--pid-file", MatchRegexp(".*"),
					handle,
				}
			}

			_, err := process.Wait() // ensure plugin has finished running before asserting on output(s)
			Expect(err).NotTo(HaveOccurred())

			Expect(readPluginArgs(argsFilepath)).To(ConsistOf(pluginArgs))
		})

		It("passes the spec serialised into a file", func() {
			processSpecFilePath := filepath.Join(client.TmpDir, "exec-process-spec")
			Eventually(processSpecFilePath).Should(BeAnExistingFile())

			readProcessSpecArg := func() (string, error) {
				var processSpec specs.Process
				if err := json.Unmarshal(readFile(processSpecFilePath), &processSpec); err != nil {
					return "", err
				}

				return processSpec.Args[0], nil
			}
			Eventually(readProcessSpecArg).Should(Equal("some-idiosyncratic-binary"))
		})

		Describe("runtime plugin stdio", func() {
			BeforeEach(func() {
				stdoutContents = "some stdout content"
				stderrContents = "some stderr content"
			})

			It("returns the runtime plugin's stdout", func() {
				Eventually(stdout).Should(gbytes.Say(stdoutContents))
			})

			It("returns the runtime plugin's stderr", func() {
				Eventually(stderr).Should(gbytes.Say(stderrContents))
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

	Describe("destroying a container", func() {
		var argsFilepath string

		JustBeforeEach(func() {
			handle = fmt.Sprintf("runtime-plugin-test-handle-%s", config.Tag)
			argsFilepath = filepath.Join(client.TmpDir, "delete-args")

			_, err := client.Create(garden.ContainerSpec{Handle: handle})
			Expect(err).ToNot(HaveOccurred())
		})

		It("executes the plugin, passing the correct args for delete on windows", func() {
			onlyOnWindows()
			Expect(client.Destroy(handle)).To(Succeed())

			Expect(readPluginArgs(argsFilepath)).To(ConsistOf(
				binaries.RuntimePlugin,
				"--debug",
				"--log", MatchRegexp(".*"),
				"--log-format", "json",
				"delete",
				handle,
			))
		})

		It("executes the plugin, passing the correct args for delete on linux", func() {
			onlyOnLinux()
			Expect(client.Destroy(handle)).To(Succeed())

			Expect(readPluginArgs(argsFilepath)).To(ConsistOf(
				binaries.RuntimePlugin,
				"--debug",
				"--root",
				getRuncRoot(),
				"--log", MatchRegexp(".*"),
				"--log-format", "json",
				"delete",
				handle,
			))
		})
	})
})

func readPluginArgs(argsFilepath string) []string {
	Eventually(argsFilepath).Should(BeAnExistingFile())
	return strings.Split(readFileString(argsFilepath), " ")
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

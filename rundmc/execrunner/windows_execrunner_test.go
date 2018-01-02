package execrunner_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("DirectExecRunner", func() {
	var (
		cmdRunner   *fake_command_runner.FakeCommandRunner
		execRunner  *execrunner.DirectExecRunner
		logger      *lagertest.TestLogger
		runtimePath = "container-runtime-path"
		cleanupFunc func() error

		spec        *runrunc.PreparedSpec
		process     garden.Process
		processIO   garden.ProcessIO
		runErr      error
		processID   string
		processPath string
		logs        string
	)

	BeforeEach(func() {
		cmdRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test-execrunner-windows")

		execRunner = &execrunner.DirectExecRunner{
			RuntimePath:   runtimePath,
			CommandRunner: cmdRunner,
			RunMode:       "exec",
		}
		processID = "process-id"
		var err error
		processPath, err = ioutil.TempDir("", "processes")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(processPath)).To(Succeed())
	})

	Describe("Run", func() {
		JustBeforeEach(func() {
			spec = &runrunc.PreparedSpec{Process: specs.Process{Cwd: "idiosyncratic-string"}}
			process, runErr = execRunner.Run(
				logger,
				processID,
				processPath,
				"handle",
				"not-used",
				0, 0,
				processIO,
				false,
				bytes.NewBufferString("some-process"),
				cleanupFunc,
			)
		})

		It("does not error", func() {
			Expect(runErr).NotTo(HaveOccurred())
		})

		It("runs the runtime plugin", func() {
			Expect(cmdRunner.StartedCommands()).To(HaveLen(1))
			Expect(cmdRunner.StartedCommands()[0].Path).To(Equal(runtimePath))
			Expect(cmdRunner.StartedCommands()[0].Args).To(ConsistOf(
				runtimePath,
				"--debug",
				"--log", filepath.Join(processPath, "exec.log"),
				"--log-format", "json",
				"exec",
				"-p", filepath.Join(processPath, "spec.json"),
				"--pid-file", MatchRegexp(".*"),
				"handle",
			))
		})

		It("writes the process spec", func() {
			actualContents, err := ioutil.ReadFile(filepath.Join(processPath, "spec.json"))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(actualContents)).To(Equal("some-process"))
		})

		Context("when the exec mode is 'run'", func() {
			BeforeEach(func() {
				execRunner.RunMode = "run"
			})

			It("executes the runtime plugin with the correct arguments", func() {
				Expect(cmdRunner.StartedCommands()).To(HaveLen(1))
				Expect(cmdRunner.StartedCommands()[0].Path).To(Equal(runtimePath))
				Expect(cmdRunner.StartedCommands()[0].Args).To(ConsistOf(
					runtimePath,
					"--debug",
					"--log", filepath.Join(processPath, "run.log"),
					"--log-format", "json",
					"run",
					"--pid-file", MatchRegexp(".*"),
					"--bundle", processPath,
					processID,
				))
			})

			Describe("cleanup", func() {
				var called bool

				BeforeEach(func() {
					called = false
					cleanupFunc = func() error {
						called = true
						return nil
					}
				})

				It("performs extra cleanup after Wait returns", func() {
					process.Wait()
					Expect(called).To(BeTrue())
				})

				Context("when the extra cleanup returns an error", func() {
					BeforeEach(func() {
						cleanupFunc = func() error {
							return errors.New("a-cleanup-error")
						}
					})

					It("logs the error", func() {
						process.Wait()
						Expect(string(logger.Buffer().Contents())).To(ContainSubstring("a-cleanup-error"))
					})
				})
			})
		})

		Describe("logging", func() {
			BeforeEach(func() {
				processID = "some-process-id"
				logs = `{"time":"2016-03-02T13:56:38Z", "level":"warning", "msg":"some-message"}
{"time":"2016-03-02T13:56:38Z", "level":"error", "msg":"some-error"}`
				cmdRunner.WhenRunning(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					ioutil.WriteFile(filepath.Join(processPath, "exec.log"), []byte(logs), 0777)
					return nil
				})
			})

			It("forwards the logs to lager", func() {
				var execLogs []lager.LogFormat
				Eventually(func() []lager.LogFormat {
					execLogs = []lager.LogFormat{}
					for _, log := range logger.Logs() {
						if log.Message == "test-execrunner-windows.execrunner.exec" {
							execLogs = append(execLogs, log)
						}
					}
					return execLogs
				}).Should(HaveLen(2))

				Expect(execLogs[0].Data).To(HaveKeyWithValue("message", "some-message"))
			})
		})

		Context("when a process ID is passed", func() {
			BeforeEach(func() {
				processID = "frank"
			})

			It("uses it", func() {
				Expect(process.ID()).To(Equal("frank"))
			})
		})

		Context("when the runtime plugin can't be started", func() {
			BeforeEach(func() {
				cmdRunner.WhenRunning(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					return errors.New("oops")
				})
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("execing runtime plugin: oops"))
			})
		})

		Context("when stdout and stderr streams are passed in", func() {
			var (
				stdout *bytes.Buffer
				stderr *bytes.Buffer
			)

			BeforeEach(func() {
				stdout = new(bytes.Buffer)
				stderr = new(bytes.Buffer)
				processIO = garden.ProcessIO{Stdout: stdout, Stderr: stderr}

				cmdRunner.WhenRunning(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					if c.Stdout == nil || c.Stderr == nil {
						return nil
					}

					_, _ = c.Stdout.Write([]byte("hello stdout"))
					_, _ = c.Stderr.Write([]byte("an error"))
					return nil
				})
			})

			It("passes them to the command", func() {
				Expect(stdout.String()).To(Equal("hello stdout"))
				Expect(stderr.String()).To(Equal("an error"))
			})
		})
	})

	Describe("Wait", func() {
		var (
			process garden.Process
			err     error
		)

		JustBeforeEach(func() {
			process, err = execRunner.Run(
				lagertest.NewTestLogger("execrunner-windows"),
				processID,
				processPath,
				"handle",
				"not-used",
				0, 0,
				garden.ProcessIO{},
				false,
				bytes.NewBufferString("some-process"),
				nil,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the process exits 0", func() {
			BeforeEach(func() {
				cmdRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					return exitWith(0).Run()
				})
			})

			It("returns the exit code of the process", func() {
				exitCode, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})
		})

		Context("when the process returns a non-zero exit code", func() {
			BeforeEach(func() {
				cmdRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					return exitWith(12).Run()
				})
			})

			It("returns the exit code of the process", func() {
				exitCode, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(12))
			})
		})

		Context("when it returns a non ExitError", func() {
			BeforeEach(func() {
				cmdRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					return errors.New("couldn't wait for process")
				})
			})

			It("returns the error", func() {
				exitCode, err := process.Wait()
				Expect(err).To(MatchError("couldn't wait for process"))
				Expect(exitCode).To(Equal(1))
			})
		})

		Context("when it is called consecutive times", func() {
			BeforeEach(func() {
				cmdRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					return exitWith(12).Run()
				})
			})

			It("returns the same exit code both times ", func() {
				exitCode, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(Equal(12))

				exitCode, err = process.Wait()
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCode).To(Equal(12))
			})
		})

		Context("when it is called multiple times in parallel before the process has exited", func() {
			proceed := make(chan struct{})

			BeforeEach(func() {
				cmdRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					<-proceed
					return exitWith(12).Run()
				})
			})

			It("returns the same exit code every time", func() {
				var wg sync.WaitGroup

				for i := 0; i < 5; i++ {
					wg.Add(1)

					go func() {
						defer wg.Done()
						defer GinkgoRecover()

						exitCode, err := process.Wait()
						Expect(err).NotTo(HaveOccurred())
						Expect(exitCode).To(Equal(12))
					}()
				}

				close(proceed)
				wg.Wait()
			})
		})
	})
})

func exitWith(exitCode int) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("powershell.exe", "-Command", fmt.Sprintf("Exit %d", exitCode))
	}

	return exec.Command("sh", "-c", fmt.Sprintf("exit %d", exitCode))
}

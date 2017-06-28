package execrunner_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("DirectExecRunner", func() {
	var (
		cmdRunner    *fake_command_runner.FakeCommandRunner
		processIDGen *runruncfakes.FakeUidGenerator
		execRunner   *execrunner.DirectExecRunner
		runtimePath  = "container-runtime-path"

		spec         *runrunc.PreparedSpec
		process      garden.Process
		processIO    garden.ProcessIO
		runErr       error
		processID    string
		processesDir string
	)

	BeforeEach(func() {
		var err error
		cmdRunner = fake_command_runner.New()
		processIDGen = new(runruncfakes.FakeUidGenerator)
		processIDGen.GenerateStub = func() string {
			return strconv.Itoa(rand.Int())
		}

		execRunner = &execrunner.DirectExecRunner{
			RuntimePath:   runtimePath,
			CommandRunner: cmdRunner,
			ProcessIDGen:  processIDGen,
		}
		processID = ""
		processesDir, err = ioutil.TempDir("", "processes")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(processesDir)).To(Succeed())
	})

	Describe("Run", func() {
		JustBeforeEach(func() {
			spec = &runrunc.PreparedSpec{Process: specs.Process{Cwd: "idiosyncratic-string"}}
			process, runErr = execRunner.Run(
				lagertest.NewTestLogger("execrunner-windows"),
				processID, spec,
				"a-bundle", processesDir, "handle",
				nil, processIO,
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
				"--log", MatchRegexp(".*"),
				"exec",
				"-p", filepath.Join(processesDir, process.ID(), "spec.json"),
				"--pid-file", MatchRegexp(".*"),
				"handle",
			))
		})

		It("writes the process spec", func() {
			actualContents, err := ioutil.ReadFile(filepath.Join(processesDir, process.ID(), "spec.json"))
			Expect(err).ToNot(HaveOccurred())
			actualSpec := &runrunc.PreparedSpec{}
			Expect(json.Unmarshal(actualContents, actualSpec)).To(Succeed())
			Expect(actualSpec).To(Equal(spec))
		})

		Context("when no process ID is passed", func() {
			BeforeEach(func() {
				processIDGen.GenerateReturns("some-generated-id")
			})

			It("uses a generated process ID", func() {
				Expect(process.ID()).To(Equal("some-generated-id"))
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

		Context("when a processID is reused concurrently", func() {
			BeforeEach(func() {
				processID = "reused"
				_, err := execRunner.Run(lagertest.NewTestLogger("execrunner-windows"), processID, &runrunc.PreparedSpec{}, "a-bundle", processesDir, "handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a sensible error", func() {
				Expect(runErr).To(MatchError(fmt.Sprintf("process ID '%s' already in use", processID)))
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
			spec := &runrunc.PreparedSpec{Process: specs.Process{Cwd: "idiosyncratic-string"}}
			process, err = execRunner.Run(
				lagertest.NewTestLogger("execrunner-windows"),
				processID, spec,
				"a-bundle", processesDir, "handle",
				nil, garden.ProcessIO{},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the process exits 0", func() {
			BeforeEach(func() {
				cmdRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
					return exec.Command("powershell.exe", "-Command", "Exit 0").Run()
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
					return exec.Command("powershell.exe", "-Command", "Exit 12").Run()
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
					return exec.Command("powershell.exe", "-Command", "Exit 12").Run()
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
					return exec.Command("powershell.exe", "-Command", "Exit 12").Run()
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

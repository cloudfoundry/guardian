package execrunner_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/execrunnerfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("WindowsExecRunner", func() {
	const (
		runtimePath = "container-runtime-path"
	)

	var (
		cmdRunner   *fake_command_runner.FakeCommandRunner
		execRunner  *execrunner.WindowsExecRunner
		logger      *lagertest.TestLogger
		cleanupFunc func() error

		process               garden.Process
		processIO             garden.ProcessIO
		runErr                error
		processID             string
		bundlePath            string
		processPath           string
		logs                  string
		wincExitCode          int
		wincWaitErrors        bool
		wincStartErrors       bool
		waitBeforeStdoutWrite bool
		proceed               chan struct{}
		bundleSaver           *depotfakes.FakeBundleSaver
		bundleLookupper       *depotfakes.FakeBundleLookupper
		processDepot          *execrunnerfakes.FakeProcessDepot
	)

	BeforeEach(func() {
		cmdRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test-execrunner-windows")

		bundleSaver = new(depotfakes.FakeBundleSaver)
		bundleLookupper = new(depotfakes.FakeBundleLookupper)
		processDepot = new(execrunnerfakes.FakeProcessDepot)

		var err error
		bundlePath, err = ioutil.TempDir("", "dadooexecrunnerbundle")
		Expect(err).NotTo(HaveOccurred())
		bundleLookupper.LookupReturns(bundlePath, nil)

		execRunner = execrunner.NewWindowsExecRunner(runtimePath, "exec", cmdRunner, bundleSaver, bundleLookupper, processDepot)
		processID = "process-id"
		processPath = filepath.Join(bundlePath, "processes", processID)
		Expect(os.MkdirAll(processPath, 0700)).To(Succeed())

		processDepot.CreateProcessDirReturns(processPath, nil)

		wincWaitErrors = false
		wincStartErrors = false
		waitBeforeStdoutWrite = false
		processIO = garden.ProcessIO{
			Stdin: strings.NewReader(""),
		}
	})

	setupCommandRunner := func(runner *fake_command_runner.FakeCommandRunner, startErrors, waitErrors, waitBeforeWrite bool, exitCode int, block chan struct{}) {
		pio := []*os.File{}
		runner.WhenStarting(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
			if startErrors {
				return errors.New("oops")
			}
			pio = duplicateStdioAndLog(c.Stdin, c.Stdout, c.Stderr, c.Args, processPath)
			return nil
		})

		runner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: runtimePath}, func(c *exec.Cmd) error {
			defer func() {
				for _, val := range pio {
					val.Close()
				}
			}()

			if block != nil {
				<-block
			}

			if waitErrors {
				return errors.New("couldn't wait for process")
			}

			if logs != "" {
				_, err := pio[3].Write([]byte(logs))
				Expect(err).NotTo(HaveOccurred())
				pio[3].Close()
			}

			// Sleep before printing any stdout/stderr to allow attach calls to complete
			if waitBeforeWrite {
				time.Sleep(time.Millisecond * 500)
			}

			_, _ = pio[1].Write([]byte("hello stdout"))

			io.Copy(pio[2], pio[0])

			if exitCode != 0 {
				return exitWith(exitCode).Run()
			}

			return nil
		})
	}

	JustBeforeEach(func() {
		setupCommandRunner(cmdRunner, wincStartErrors, wincWaitErrors, waitBeforeStdoutWrite, wincExitCode, proceed)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(processPath)).To(Succeed())
	})

	Describe("Run", func() {
		JustBeforeEach(func() {
			process, runErr = execRunner.Run(
				logger,
				processID,
				"handle",
				processIO,
				false,
				bytes.NewBufferString("some-process"),
				cleanupFunc,
			)
		})

		It("does not error", func() {
			Expect(runErr).NotTo(HaveOccurred())
		})

		It("creates the process folder in the depot", func() {
			Expect(processDepot.CreateProcessDirCallCount()).To(Equal(1))
			_, actualSandboxHandle, actualProcessID := processDepot.CreateProcessDirArgsForCall(0)
			Expect(actualSandboxHandle).To(Equal("handle"))
			Expect(actualProcessID).To(Equal(processID))
		})

		It("runs the runtime plugin", func() {
			Expect(cmdRunner.StartedCommands()).To(HaveLen(1))
			Expect(cmdRunner.StartedCommands()[0].Path).To(Equal(runtimePath))
			Expect(cmdRunner.StartedCommands()[0].Args).To(ConsistOf(
				runtimePath,
				"--debug",
				"--log-handle", MatchRegexp("\\d+"),
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

		When("creating the process folder in the depot fails", func() {
			BeforeEach(func() {
				processDepot.CreateProcessDirReturns("", errors.New("create-process-dir-error"))
			})

			It("fails", func() {
				Expect(runErr).To(MatchError("create-process-dir-error"))
			})
		})

		Describe("logging", func() {
			BeforeEach(func() {
				processID = "some-process-id"
				logs = `{"time":"2016-03-02T13:56:38Z", "level":"warning", "msg":"some-message"}
{"time":"2016-03-02T13:56:38Z", "level":"error", "msg":"some-error"}`
			})

			It("forwards the logs to lager", func() {
				var execLogs []lager.LogFormat
				Eventually(func() []lager.LogFormat {
					execLogs = []lager.LogFormat{}
					for _, log := range logger.Logs() {
						if log.Message == "test-execrunner-windows.execrunner.winc" {
							execLogs = append(execLogs, log)
						}
					}
					return execLogs
				}).Should(HaveLen(2))

				Expect(execLogs[0].Data).To(HaveKeyWithValue("message", "some-message"))
			})

			It("the gofunc streaming logs exits", func() {
				Eventually(func() bool {
					found := false
					for _, log := range logger.Logs() {
						if log.Message == "test-execrunner-windows.execrunner.done-streaming-winc-logs" {
							found = true
						}
					}
					return found
				}).Should(BeTrue())
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
				wincStartErrors = true
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("execing runtime plugin: oops"))
			})
		})

		Context("when stdin, stdout, and stderr streams are passed in", func() {
			var (
				stdout *bytes.Buffer
				stderr *bytes.Buffer
			)

			BeforeEach(func() {
				stdout = new(bytes.Buffer)
				stderr = new(bytes.Buffer)
				processIO = garden.ProcessIO{Stdin: strings.NewReader("omg"), Stdout: stdout, Stderr: stderr}

			})

			It("passes them to the command", func() {
				_, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(stdout.String()).To(Equal("hello stdout"))
				Expect(stderr.String()).To(Equal("omg"))
			})
		})
	})

	Describe("RunPea", func() {
		JustBeforeEach(func() {
			execRunner = execrunner.NewWindowsExecRunner(runtimePath, "run", cmdRunner, bundleSaver, bundleLookupper, processDepot)
			process, runErr = execRunner.RunPea(
				logger,
				processID,
				goci.Bndl{Spec: specs.Spec{Version: "my-bundle"}},
				"handle",
				processIO,
				false,
				bytes.NewBufferString("some-process"),
				cleanupFunc,
			)
		})

		It("succeeds", func() {
			Expect(runErr).NotTo(HaveOccurred())
		})

		It("executes the runtime plugin with the correct arguments", func() {
			Expect(cmdRunner.StartedCommands()).To(HaveLen(1))
			Expect(cmdRunner.StartedCommands()[0].Path).To(Equal(runtimePath))
			Expect(cmdRunner.StartedCommands()[0].Args).To(ConsistOf(
				runtimePath,
				"--debug",
				"--log-handle", MatchRegexp("\\d+"),
				"--log-format", "json",
				"run",
				"--pid-file", MatchRegexp(".*"),
				"--bundle", processPath,
				processID,
			))
		})

		It("creates the process folder in the depot", func() {
			Expect(processDepot.CreateProcessDirCallCount()).To(Equal(1))
			_, actualSandboxHandle, actualProcessID := processDepot.CreateProcessDirArgsForCall(0)
			Expect(actualSandboxHandle).To(Equal("handle"))
			Expect(actualProcessID).To(Equal(processID))
		})

		It("writes the bundle spec", func() {
			Expect(bundleSaver.SaveCallCount()).To(Equal(1))
			actualBundle, actualPath := bundleSaver.SaveArgsForCall(0)
			Expect(actualBundle.Spec.Version).To(Equal("my-bundle"))
			Expect(actualPath).To(Equal(processPath))
		})

		When("creating the process folder in the depot fails", func() {
			BeforeEach(func() {
				processDepot.CreateProcessDirReturns("", errors.New("create-process-dir-error"))
			})

			It("fails", func() {
				Expect(runErr).To(MatchError("create-process-dir-error"))
			})
		})

		When("saving the bundle in the depot fails", func() {
			BeforeEach(func() {
				bundleSaver.SaveReturns(errors.New("save-error"))
			})

			It("fails", func() {
				Expect(runErr).To(MatchError("save-error"))
			})
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

		Describe("logging", func() {
			BeforeEach(func() {
				processID = "some-process-id"
				logs = `{"time":"2016-03-02T13:56:38Z", "level":"warning", "msg":"some-message"}
{"time":"2016-03-02T13:56:38Z", "level":"error", "msg":"some-error"}`
			})

			It("forwards the logs to lager", func() {
				var execLogs []lager.LogFormat
				Eventually(func() []lager.LogFormat {
					execLogs = []lager.LogFormat{}
					for _, log := range logger.Logs() {
						if log.Message == "test-execrunner-windows.execrunner.winc" {
							execLogs = append(execLogs, log)
						}
					}
					return execLogs
				}).Should(HaveLen(2))

				Expect(execLogs[0].Data).To(HaveKeyWithValue("message", "some-message"))
			})

			It("the gofunc streaming logs exits", func() {
				Eventually(func() bool {
					found := false
					for _, log := range logger.Logs() {
						if log.Message == "test-execrunner-windows.execrunner.done-streaming-winc-logs" {
							found = true
						}
					}
					return found
				}).Should(BeTrue())
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
				wincStartErrors = true
			})

			It("returns an error", func() {
				Expect(runErr).To(MatchError("execing runtime plugin: oops"))
			})
		})

		Context("when stdin, stdout, and stderr streams are passed in", func() {
			var (
				stdout *bytes.Buffer
				stderr *bytes.Buffer
			)

			BeforeEach(func() {
				stdout = new(bytes.Buffer)
				stderr = new(bytes.Buffer)
				processIO = garden.ProcessIO{Stdin: strings.NewReader("omg"), Stdout: stdout, Stderr: stderr}

			})

			It("passes them to the command", func() {
				_, err := process.Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(stdout.String()).To(Equal("hello stdout"))
				Expect(stderr.String()).To(Equal("omg"))
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
				"handle",
				processIO,
				false,
				bytes.NewBufferString("some-process"),
				nil,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the process exits 0", func() {
			BeforeEach(func() {
				wincExitCode = 0
			})

			It("returns the exit code of the process", func() {
				exitCode, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})
		})

		Context("when the process returns a non-zero exit code", func() {
			BeforeEach(func() {
				wincExitCode = 12
			})

			It("returns the exit code of the process", func() {
				exitCode, err := process.Wait()
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(12))
			})
		})

		Context("when it returns a non ExitError", func() {
			BeforeEach(func() {
				wincWaitErrors = true
			})

			It("returns the error", func() {
				exitCode, err := process.Wait()
				Expect(err).To(MatchError("couldn't wait for process"))
				Expect(exitCode).To(Equal(1))
			})
		})

		Context("when it is called consecutive times", func() {
			BeforeEach(func() {
				wincExitCode = 12
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

			BeforeEach(func() {
				proceed = make(chan struct{})
				wincExitCode = 12
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

	Describe("Attach", func() {
		BeforeEach(func() {
			wincExitCode = 8
			waitBeforeStdoutWrite = true

			var buffer bytes.Buffer
			for i := 0; i < 10240; i++ {
				buffer.WriteString("a")
			}
			processIO = garden.ProcessIO{Stdin: strings.NewReader(buffer.String()), Stderr: new(bytes.Buffer)}
		})

		JustBeforeEach(func() {
			process, runErr = execRunner.Run(
				logger,
				processID,
				"handle",
				processIO,
				false,
				bytes.NewBufferString("some-process"),
				cleanupFunc,
			)
			Expect(runErr).NotTo(HaveOccurred())
		})

		It("returns the process with the given id", func() {
			p, err := execRunner.Attach(logger, processID, garden.ProcessIO{}, "")
			Expect(err).NotTo(HaveOccurred())

			Expect(p).To(Equal(process))
		})

		It("calling Wait on the returned process gives the exit code", func() {
			p, err := execRunner.Attach(logger, processID, garden.ProcessIO{}, "")
			Expect(err).NotTo(HaveOccurred())

			code, err := p.Wait()
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(Equal(8))
		})

		It("pass stdin to, and get the stdout and stderr from the attached process", func() {
			aout := new(bytes.Buffer)
			aerr := new(bytes.Buffer)

			attachIO := garden.ProcessIO{
				Stdin:  strings.NewReader("omg"),
				Stdout: aout,
				Stderr: aerr,
			}

			p, err := execRunner.Attach(logger, processID, attachIO, "")
			Expect(err).NotTo(HaveOccurred())

			_, err = p.Wait()
			Expect(err).NotTo(HaveOccurred())

			Expect(aout.String()).To(Equal("hello stdout"))
			Expect(aerr.String()).To(ContainSubstring("omg"))
		})

		Context("multiple processes streaming stdout and stderr", func() {
			var (
				runStdout *bytes.Buffer
				runStderr *bytes.Buffer
			)

			BeforeEach(func() {
				runStdout = new(bytes.Buffer)
				runStderr = new(bytes.Buffer)

				processIO = garden.ProcessIO{
					Stdin:  strings.NewReader("omg"),
					Stdout: runStdout,
					Stderr: runStderr,
				}
			})

			It("multiplexes stdout and stderr", func() {
				attachStdout := new(bytes.Buffer)
				attachStderr := new(bytes.Buffer)

				_, err := execRunner.Attach(logger, processID, garden.ProcessIO{Stdout: attachStdout, Stderr: attachStderr}, "")
				Expect(err).NotTo(HaveOccurred())

				process.Wait()
				Expect(runStdout.String()).To(Equal("hello stdout"))
				Expect(attachStdout.String()).To(Equal("hello stdout"))

				Expect(runStderr.String()).To(Equal("omg"))
				Expect(attachStderr.String()).To(Equal("omg"))
			})
		})

		Context("a process with the given id does not exist", func() {
			It("returns a ProcessNotFound error", func() {
				_, err := execRunner.Attach(logger, "does-not-exist", garden.ProcessIO{}, "")
				Expect(err).To(MatchError(garden.ProcessNotFoundError{ProcessID: "does-not-exist"}))
			})
		})
	})
})

func exitWith(exitCode int) *exec.Cmd {
	return exec.Command("cmd.exe", "/c", fmt.Sprintf("Exit %d", exitCode))
}

func duplicateStdioAndLog(stdin io.Reader, stdout io.Writer, stderr io.Writer, args []string, processPath string) []*os.File {
	var origFiles [4]uintptr

	fstdin, ok := stdin.(*os.File)
	ExpectWithOffset(1, ok).To(BeTrue())
	origFiles[0] = fstdin.Fd()

	fstdout, ok := stdout.(*os.File)
	ExpectWithOffset(1, ok).To(BeTrue())
	origFiles[1] = fstdout.Fd()

	fstderr, ok := stderr.(*os.File)
	ExpectWithOffset(1, ok).To(BeTrue())
	origFiles[2] = fstderr.Fd()

	var handle uint64
	var err error

	for i, v := range args {
		if v == "--log-handle" {
			handle, err = strconv.ParseUint(args[i+1], 10, 64)
			Expect(err).NotTo(HaveOccurred())

			break
		}
	}
	Expect(handle).NotTo(Equal(uint64(0)))

	origFiles[3] = uintptr(handle)

	var duplicates []*os.File

	ExpectWithOffset(1, ok).To(BeTrue())

	var dupped syscall.Handle
	self, _ := syscall.GetCurrentProcess()

	for i, h := range origFiles {
		err := syscall.DuplicateHandle(self, syscall.Handle(h), self, &dupped, 0, false, syscall.DUPLICATE_SAME_ACCESS)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		duplicates = append(duplicates, os.NewFile(uintptr(dupped), fmt.Sprintf("%s.%d", processPath, i)))
	}

	return duplicates
}

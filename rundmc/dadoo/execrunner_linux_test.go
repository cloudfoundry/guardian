package dadoo_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/dadoo"
	dadoofakes "code.cloudfoundry.org/guardian/rundmc/dadoo/dadoofakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Dadoo ExecRunner", func() {
	var (
		fakeCommandRunner      *fake_command_runner.FakeCommandRunner
		fakeProcessIDGenerator *fakes.FakeUidGenerator
		fakePidGetter          *dadoofakes.FakePidGetter
		runner                 *dadoo.ExecRunner
		bundlePath             string
		processPath            string
		pidPath                string
		receivedStdinContents  []byte
		runcReturns            byte
		dadooReturns           error
		dadooWritesLogs        string
		dadooWritesExitCode    []byte
		log                    *lagertest.TestLogger
		receiveWinSize         func(*os.File)
		closeExitPipeCh        chan struct{}

		testFinishedCh chan bool
	)

	BeforeEach(func() {
		testFinishedCh = make(chan bool)

		fakeCommandRunner = fake_command_runner.New()
		fakeProcessIDGenerator = new(fakes.FakeUidGenerator)
		fakePidGetter = new(dadoofakes.FakePidGetter)

		fakeProcessIDGenerator.GenerateReturns("the-pid")
		fakePidGetter.PidReturns(0, nil)

		var err error
		bundlePath, err = ioutil.TempDir("", "dadooexecrunnerbundle")
		Expect(err).NotTo(HaveOccurred())
		processPath = filepath.Join(bundlePath, "the-process")
		pidPath = filepath.Join(processPath, "0.pid")

		runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", fakeProcessIDGenerator, fakePidGetter, fakeCommandRunner)
		log = lagertest.NewTestLogger("test")

		runcReturns = 0
		dadooReturns = nil
		dadooWritesExitCode = []byte("0")
		dadooWritesLogs = `time="2016-03-02T13:56:38Z" level=warning msg="signal: potato"
				time="2016-03-02T13:56:38Z" level=error msg="fork/exec POTATO: no such file or directory"
				time="2016-03-02T13:56:38Z" level=fatal msg="Container start failed: [10] System error: fork/exec POTATO: no such file or directory"`

		dadooFlags := flag.NewFlagSet("something", flag.PanicOnError)
		dadooFlags.Bool("tty", false, "")
		dadooFlags.Int("uid", 0, "")
		dadooFlags.Int("gid", 0, "")
		dadooFlags.Int("rows", 0, "")
		dadooFlags.Int("cols", 0, "")

		receiveWinSize = func(_ *os.File) {}

		closeExitPipeCh = make(chan struct{})
		close(closeExitPipeCh) // default to immediately succeeding

		// dadoo should open up its end of the named pipes
		fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "path-to-dadoo",
		}, func(cmd *exec.Cmd) error {
			var err error
			receivedStdinContents, err = ioutil.ReadAll(cmd.Stdin)
			Expect(err).NotTo(HaveOccurred())

			// dup the fd so that the runner is allowed to close it
			// in a real fork/exec this'd happen as part of the fork
			fd3 := dup(cmd.ExtraFiles[0])
			fd4 := dup(cmd.ExtraFiles[1])

			if dadooReturns != nil {
				// dadoo would error - bail out
				return dadooReturns
			}

			// dadoo would not error - simulate dadoo operation
			go func(cmd *exec.Cmd, exitCode []byte, logs []byte, closeExitPipeCh chan struct{}, recvWinSz func(*os.File)) {
				defer GinkgoRecover()

				// parse flags to get bundle dir argument so we can open stdin/out/err pipes
				dadooFlags.Parse(cmd.Args[1:])
				processDir := dadooFlags.Arg(2)
				si, so, se, winsz, exit := openPipes(processDir)

				go recvWinSz(winsz)

				// write log file to fd4
				_, err = io.Copy(fd4, bytes.NewReader([]byte(logs)))
				Expect(err).NotTo(HaveOccurred())
				fd4.Close()

				// return exit status of runc on fd3
				_, err = fd3.Write([]byte{runcReturns})
				Expect(err).NotTo(HaveOccurred())
				fd3.Close()

				// write exit code of actual process to $procesdir/exitcode file
				if exitCode != nil {
					Expect(ioutil.WriteFile(filepath.Join(processDir, "exitcode"), []byte(exitCode), 0600)).To(Succeed())
				}

				<-closeExitPipeCh
				Expect(exit.Close()).To(Succeed())

				// do some test IO (directly write to stdout and copy stdin->stderr)
				so.WriteString("hello stdout")

				// it's a trap - stdin is copied to stderr so that we can test
				// its content further down
				_, err = io.Copy(se, si)
				Expect(err).NotTo(HaveOccurred())
				se.WriteString("done copying stdin")

				Expect(so.Close()).To(Succeed())
				Expect(se.Close()).To(Succeed())

			}(cmd, dadooWritesExitCode, []byte(dadooWritesLogs), closeExitPipeCh, receiveWinSize)

			return nil
		})
	})

	Describe("Run", func() {
		It("executes the dadoo binary with the correct arguments", func() {
			runner.Run(log, &runrunc.PreparedSpec{}, processPath, "some-handle", nil, garden.ProcessIO{})

			Expect(fakeCommandRunner.StartedCommands()[0].Args).To(
				ConsistOf(
					"path-to-dadoo",
					"exec", "path-to-runc", filepath.Join(processPath, "the-pid"), "some-handle",
				),
			)
		})

		Context("when TTY is requested", func() {
			It("executed the dadoo binary with the correct arguments", func() {
				runner.Run(log, &runrunc.PreparedSpec{
					HostUID: 123,
					HostGID: 456,
				},
					processPath,
					"some-handle",
					&garden.TTYSpec{
						WindowSize: &garden.WindowSize{
							Rows:    12,
							Columns: 13,
						},
					},
					garden.ProcessIO{},
				)

				Expect(fakeCommandRunner.StartedCommands()[0].Args).To(
					Equal([]string{
						"path-to-dadoo",
						"-tty",
						"-rows", "12",
						"-cols", "13",
						"-uid", "123",
						"-gid", "456",
						"exec", "path-to-runc", filepath.Join(processPath, "the-pid"), "some-handle",
					}),
				)
			})
		})

		It("does not block on dadoo returning before returning", func() {
			waitBlocks := make(chan struct{})
			defer close(waitBlocks)

			fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: "path-to-dadoo"}, func(cmd *exec.Cmd) error {
				<-waitBlocks
				return nil
			})

			runReturns := make(chan struct{})
			go func(runner *dadoo.ExecRunner) {
				defer GinkgoRecover()
				runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
				close(runReturns)
			}(runner)

			Eventually(runReturns).Should(BeClosed())

			Expect(fakeCommandRunner.StartedCommands()).To(HaveLen(1))
			Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(0))
			Eventually(fakeCommandRunner.WaitedCommands).Should(ConsistOf(fakeCommandRunner.StartedCommands())) // avoid zombies by waiting
		})

		It("passes the encoded process spec on STDIN of dadoo", func() {
			runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
			Expect(string(receivedStdinContents)).To(ContainSubstring(`"args":["Banana","rama"]`))
			Expect(string(receivedStdinContents)).NotTo(ContainSubstring(`HostUID`))
		})

		It("cleans up the processes dir after Wait returns", func() {
			process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())

			Expect(filepath.Join(processPath, "the-pid")).To(BeAnExistingFile())

			_, err = process.Wait()
			Expect(err).NotTo(HaveOccurred())

			Expect(filepath.Join(processPath, "the-pid")).NotTo(BeAnExistingFile())
		})

		Context("when spawning dadoo fails", func() {
			It("returns a nice error", func() {
				dadooReturns = errors.New("boom")

				_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})

		Describe("Logging", func() {
			It("sends all the logs to the logger", func() {
				_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				runcLogs := make([]lager.LogFormat, 0)
				for _, log := range log.Logs() {
					if log.Message == "test.execrunner.runc" {
						runcLogs = append(runcLogs, log)
					}
				}

				Expect(runcLogs).To(HaveLen(3))
				Expect(runcLogs[0].Data).To(HaveKeyWithValue("message", "signal: potato"))
			})

			Context("when `runC exec` fails", func() {
				BeforeEach(func() {
					runcReturns = 3
				})

				It("return an error including parsed logs when runC fails to start the container", func() {
					_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).To(MatchError("runc exec: exit status 3: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
				})

				Context("when the log messages can't be parsed", func() {
					BeforeEach(func() {
						dadooWritesLogs = `foo="'
					`
					})

					It("returns an error with only the exit status if the log can't be parsed", func() {
						_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).To(MatchError("runc exec: exit status 3: "))
					})
				})
			})
		})

		Describe("the returned garden.Process", func() {
			It("has the correct ID", func() {
				process, err := runner.Run(log, &runrunc.PreparedSpec{}, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				Expect(process.ID()).To(Equal("the-pid"))
			})

			Describe("SetTTY", func() {
				BeforeEach(func() {
					closeExitPipeCh = make(chan struct{})
				})

				AfterEach(func() {
					close(closeExitPipeCh)
				})

				It("sends the new window size via the winsz pipe", func() {
					var receivedWinSize garden.WindowSize

					received := make(chan struct{})
					receiveWinSize = func(winSizeFifo *os.File) {
						defer GinkgoRecover()

						err := json.NewDecoder(winSizeFifo).Decode(&receivedWinSize)
						Expect(err).NotTo(HaveOccurred())
						close(received)
					}

					process, err := runner.Run(log, &runrunc.PreparedSpec{
						HostUID: 123,
						HostGID: 456,
					},
						processPath,
						"some-handle",
						&garden.TTYSpec{
							WindowSize: &garden.WindowSize{
								Columns: 13,
								Rows:    17,
							},
						},
						garden.ProcessIO{},
					)
					Expect(err).NotTo(HaveOccurred())

					process.SetTTY(garden.TTYSpec{WindowSize: &garden.WindowSize{Columns: 53, Rows: 59}})

					Eventually(received, "5s").Should(BeClosed())
					Expect(receivedWinSize).To(Equal(
						garden.WindowSize{
							Columns: 53,
							Rows:    59,
						},
					))
				})
			})

			Describe("Signal", func() {
				It("reads the PID from the pid file", func() {
					process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					process.Signal(garden.SignalTerminate)
					Expect(fakePidGetter.PidArgsForCall(0)).To(Equal(filepath.Join(processPath, "the-pid", "pidfile")))
				})

				Context("when the pidGetter returns an error", func() {
					BeforeEach(func() {
						fakePidGetter.PidReturns(0, errors.New("Unable to get PID"))
					})

					It("returns an appropriate error", func() {
						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						Expect(process.Signal(garden.SignalTerminate)).To(MatchError("fetching-pid: Unable to get PID"))
					})
				})

				Context("when there is a process running", func() {
					var (
						cmd  *exec.Cmd
						sess *gexec.Session
					)

					BeforeEach(func() {
						var err error

						cmd = exec.Command("sh", "-c", "trap 'exit 41' TERM; while true; do echo trapping; sleep 1; done")
						sess, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
						Expect(err).NotTo(HaveOccurred())

						Eventually(sess).Should(gbytes.Say("trapping"))
					})

					It("gets signalled", func() {
						process, err := runner.Run(
							log,
							&runrunc.PreparedSpec{
								Process: specs.Process{
									Args: []string{"echo", "This won't actually do anything as the command runner is faked"},
								},
							},
							processPath,
							"some-handle",
							nil,
							garden.ProcessIO{},
						)
						Expect(err).NotTo(HaveOccurred())

						fakePidGetter.PidReturns(cmd.Process.Pid, nil)
						Expect(process.Signal(garden.SignalTerminate)).To(Succeed())

						Eventually(sess, "5s").Should(gexec.Exit(41))
					})
				})

				Context("when os.Signal returns an error", func() {
					BeforeEach(func() {
						fakePidGetter.PidReturns(0, nil)
					})

					It("forwards the error", func() {
						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", ""}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						Expect(process.Signal(garden.SignalTerminate)).To(MatchError("os: process not initialized"))
					})
				})
			})

			Describe("Wait", func() {
				Context("when the process does not exit immediately", func() {
					BeforeEach(func() {
						closeExitPipeCh = make(chan struct{})
					})

					It("does not return until the exit pipe is closed", func() {
						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						done := make(chan struct{})
						go func() {
							process.Wait()
							close(done)
						}()

						Consistently(done).ShouldNot(BeClosed())
						close(closeExitPipeCh)
						Eventually(done).Should(BeClosed())
					})
				})

				It("returns the exit code of the dadoo process", func() {
					dadooWritesExitCode = []byte("42")

					process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					Expect(process.Wait()).To(Equal(42))
				})

				Context("when the exitfile is empty", func() {
					It("returns an error", func() {
						dadooWritesExitCode = []byte("")

						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).To(MatchError(ContainSubstring("the exitcode file is empty")))
					})
				})

				Context("when the exitfile does not exist", func() {
					It("returns an error", func() {
						dadooWritesExitCode = nil

						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).To(MatchError(ContainSubstring("could not find the exitcode file for the process")))
					})
				})

				Context("when the exitcode file doesn't contain an exit code", func() {
					It("returns an error", func() {
						dadooWritesExitCode = []byte("potato")

						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).To(MatchError(ContainSubstring("failed to parse exit code")))
					})
				})
			})
		})

		It("can get stdout/err from the spawned process via named pipes", func() {
			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()
			process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", "ohai"}}}, processPath, "some-handle", nil, garden.ProcessIO{
				Stdout: stdout,
				Stderr: stderr,
				Stdin:  strings.NewReader("omg"),
			})
			Expect(err).NotTo(HaveOccurred())

			process.Wait()
			Eventually(stdout).Should(gbytes.Say("hello stdout"))
			Eventually(stderr).Should(gbytes.Say("omg"))
		})

		It("closed stdin when the stdin stream ends", func() {
			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()
			process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", "ohai"}}}, processPath, "some-handle", nil, garden.ProcessIO{
				Stdout: stdout,
				Stderr: stderr,
				Stdin:  strings.NewReader("omg"),
			})
			Expect(err).NotTo(HaveOccurred())

			process.Wait()
			Eventually(stderr).Should(gbytes.Say("done copying stdin"))
		})

		It("does not return from wait until all stdout/err data has been copied over", func() {
			stdinR, stdinW, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())

			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()
			process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", "ohai"}}}, processPath, "some-handle", nil, garden.ProcessIO{
				Stdout: stdout,
				Stderr: stderr,
				Stdin:  stdinR,
			})
			Expect(err).NotTo(HaveOccurred())

			done := make(chan struct{})
			go func() {
				process.Wait()
				close(done)
			}()

			Consistently(done).ShouldNot(BeClosed())
			Expect(stdinW.Close()).To(Succeed()) // closing stdin stops the copy to stderr
			Eventually(done).Should(BeClosed())
		})
	})

	Describe("Attach", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(processPath, "some-process-id"), 0700))
			Expect(syscall.Mkfifo(filepath.Join(processPath, "some-process-id", "stdin"), 0)).To(Succeed())
			Expect(syscall.Mkfifo(filepath.Join(processPath, "some-process-id", "stdout"), 0)).To(Succeed())
			Expect(syscall.Mkfifo(filepath.Join(processPath, "some-process-id", "stderr"), 0)).To(Succeed())
			Expect(syscall.Mkfifo(filepath.Join(processPath, "some-process-id", "winsz"), 0)).To(Succeed())
			Expect(syscall.Mkfifo(filepath.Join(processPath, "some-process-id", "exit"), 0)).To(Succeed())
		})

		Context("when dadoo has already exited", func() {
			It("returns the process", func() {
				out := gbytes.NewBuffer()
				process, err := runner.Attach(log, "some-process-id", garden.ProcessIO{
					Stdout: out,
				}, processPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(process).NotTo(BeNil())
			})
		})

		Context("when dadoo is running", func() {

			var stdin, stdout, stderr, exit *os.File
			var gstdin, gstdout, gstderr *os.File

			var openNonBlocking = func(fileName string) (*os.File, error) {
				file, err := os.OpenFile(fileName, os.O_RDONLY|syscall.O_NONBLOCK, 0600)
				if err != nil {
					return nil, err
				}
				if err = syscall.SetNonblock(int(file.Fd()), false); err != nil {
					return nil, err
				}
				return file, nil
			}

			JustBeforeEach(func() {

				var err error
				// pretend we opened pipes on garden
				gstdin, err = os.OpenFile(filepath.Join(processPath, "some-process-id", "stdin"), os.O_RDWR, 0600)
				Expect(err).NotTo(HaveOccurred())

				gstdout, err = openNonBlocking(filepath.Join(processPath, "some-process-id", "stdout"))
				Expect(err).NotTo(HaveOccurred())

				gstderr, err = openNonBlocking(filepath.Join(processPath, "some-process-id", "stderr"))
				Expect(err).NotTo(HaveOccurred())

				// open dadoo pipes
				stdin, stdout, stderr, _, exit = openPipes(filepath.Join(processPath, "some-process-id"))
			})

			AfterEach(func() {
				Expect(gstdin.Close()).To(Succeed())
				Expect(gstdout.Close()).To(Succeed())
				Expect(gstderr.Close()).To(Succeed())
			})

			Context("and the process doesn't immediately write to stdout or stderr", func() {

				var outBuf, errBuf *gbytes.Buffer

				JustBeforeEach(func() {
					outBuf = gbytes.NewBuffer()
					errBuf = gbytes.NewBuffer()

					_, err := runner.Attach(log, "some-process-id", garden.ProcessIO{
						Stdout: outBuf,
						Stderr: errBuf,
					}, processPath)
					Expect(err).NotTo(HaveOccurred())
				})

				It("waits for stdout", func() {
					Consistently(outBuf.Contents()).Should(BeEmpty())

					_, err := stdout.WriteString("some-text")
					Expect(err).NotTo(HaveOccurred())

					Eventually(outBuf).Should(gbytes.Say("some-text"))
				})

				It("waits for stderr", func() {
					Consistently(errBuf.Contents()).Should(BeEmpty())

					_, err := stderr.WriteString("some-text")
					Expect(err).NotTo(HaveOccurred())

					Eventually(errBuf).Should(gbytes.Say("some-text"))
				})
			})

			Context("and the process is already writing to stdout and stderr", func() {
				JustBeforeEach(func() {
					_, err := stdout.WriteString("potato")
					Expect(err).NotTo(HaveOccurred())

					_, err = stderr.WriteString("tomato")
					Expect(err).NotTo(HaveOccurred())
				})

				It("reports the correct pid", func() {
					process, err := runner.Attach(log, "some-process-id", garden.ProcessIO{}, processPath)
					Expect(err).NotTo(HaveOccurred())

					Expect(process.ID()).To(Equal("some-process-id"))
				})

				It("reattaches to the stdout output", func() {
					outBuf := gbytes.NewBuffer()
					_, err := runner.Attach(log, "some-process-id", garden.ProcessIO{
						Stdout: outBuf,
					}, processPath)
					Expect(err).NotTo(HaveOccurred())

					Eventually(outBuf).Should(gbytes.Say("potato"))
				})

				It("reattaches to the stderr output", func() {
					errBuf := gbytes.NewBuffer()
					_, err := runner.Attach(log, "some-process-id", garden.ProcessIO{
						Stderr: errBuf,
					}, processPath)
					Expect(err).NotTo(HaveOccurred())

					Eventually(errBuf).Should(gbytes.Say("tomato"))
				})

				It("reattaches to the stdin", func() {
					_, err := runner.Attach(log, "some-process-id", garden.ProcessIO{
						Stdin: strings.NewReader("hello stdin"),
					}, processPath)
					Expect(err).NotTo(HaveOccurred())

					stdinContents := gbytes.NewBuffer()
					go func() {
						io.Copy(stdinContents, stdin)
					}()

					Eventually(stdinContents).Should(gbytes.Say("hello stdin"))
				})
			})
		})
	})
})

type fakeExitError int
type fakeWaitStatus fakeExitError

func (e fakeExitError) Error() string {
	return fmt.Sprintf("Fake Exit Error: %d", e)
}

func (e fakeExitError) Sys() interface{} {
	return fakeWaitStatus(e)
}

func (w fakeWaitStatus) ExitStatus() int {
	return int(w)
}

func dup(f *os.File) *os.File {
	dupped, err := syscall.Dup(int(f.Fd()))
	Expect(err).NotTo(HaveOccurred())
	return os.NewFile(uintptr(dupped), f.Name()+"dup")
}

func openPipes(dir string) (stdin, stdout, stderr, winsz, exit *os.File) {
	si, err := os.OpenFile(filepath.Join(dir, "stdin"), os.O_RDONLY, 0600)
	Expect(err).NotTo(HaveOccurred())

	so, err := os.OpenFile(filepath.Join(dir, "stdout"), os.O_APPEND|os.O_RDWR, 0600)
	Expect(err).NotTo(HaveOccurred())

	se, err := os.OpenFile(filepath.Join(dir, "stderr"), os.O_APPEND|os.O_RDWR, 0600)
	Expect(err).NotTo(HaveOccurred())

	exit, err = os.OpenFile(filepath.Join(dir, "exit"), os.O_APPEND|os.O_RDWR, 0600)
	Expect(err).NotTo(HaveOccurred())

	winsz, err = os.OpenFile(filepath.Join(dir, "winsz"), os.O_RDWR, 0600)
	Expect(err).NotTo(HaveOccurred())

	return si, so, se, winsz, exit
}

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
	"time"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/dadoo/dadoofakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Dadoo ExecRunner", func() {
	var (
		fakeCommandRunner                      *fake_command_runner.FakeCommandRunner
		fakeProcessIDGenerator                 *fakes.FakeUidGenerator
		fakePidGetter                          *dadoofakes.FakePidGetter
		runner                                 *dadoo.ExecRunner
		bundlePath                             string
		processPath                            string
		processID                              string
		pidPath                                string
		receivedStdinContents                  []byte
		runcReturns                            byte
		dadooReturns                           error
		runcHangsForEver                       bool
		dadooPanicsBeforeReportingRuncExitCode bool
		dadooWritesLogs                        string
		dadooWritesExitCode                    []byte
		log                                    *lagertest.TestLogger
		receiveWinSize                         func(*os.File)
		closeExitPipeCh                        chan struct{}
		stderrContents                         string
	)

	BeforeEach(func() {
		fakeCommandRunner = fake_command_runner.New()
		fakeProcessIDGenerator = new(fakes.FakeUidGenerator)
		fakePidGetter = new(dadoofakes.FakePidGetter)

		processID = fmt.Sprintf("pid-%d", GinkgoParallelNode())
		fakePidGetter.PidReturns(0, nil)

		var err error
		bundlePath, err = ioutil.TempDir("", "dadooexecrunnerbundle")
		Expect(err).NotTo(HaveOccurred())
		processPath = filepath.Join(bundlePath, "the-process")
		pidPath = filepath.Join(processPath, "0.pid")

		runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", "runc-root", fakeProcessIDGenerator, fakePidGetter, fakeCommandRunner, false)
		log = lagertest.NewTestLogger("test")

		runcReturns = 0
		dadooReturns = nil
		runcHangsForEver = false
		dadooPanicsBeforeReportingRuncExitCode = false
		dadooWritesExitCode = []byte("0")
		dadooWritesLogs = `time="2016-03-02T13:56:38Z" level=warning msg="signal: potato"
				time="2016-03-02T13:56:38Z" level=error msg="fork/exec POTATO: no such file or directory"
				time="2016-03-02T13:56:38Z" level=fatal msg="Container start failed: [10] System error: fork/exec POTATO: no such file or directory"`

		dadooFlags := flag.NewFlagSet("something", flag.PanicOnError)
		dadooFlags.Bool("tty", false, "")
		dadooFlags.Int("rows", 0, "")
		dadooFlags.Int("cols", 0, "")
		dadooFlags.String("runc-root", "", "")

		receiveWinSize = func(_ *os.File) {}

		closeExitPipeCh = make(chan struct{})
		close(closeExitPipeCh) // default to immediately succeeding

		stderrContents = ""

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
			fd5 := dup(cmd.ExtraFiles[2])

			if dadooReturns != nil {
				// dadoo would error - bail out
				return dadooReturns
			}

			// dadoo might talk
			fmt.Fprintln(cmd.Stdout, "dadoo stdout")
			fmt.Fprintln(cmd.Stderr, "dadoo stderr")

			// dadoo would not error - simulate dadoo operation
			go func(cmd *exec.Cmd, runcHangsForEver, dadooPanicsBeforeReportingRuncExitCode bool, exitCode []byte, logs []byte, closeExitPipeCh chan struct{}, recvWinSz func(*os.File), stderrContents string) {
				defer GinkgoRecover()

				// parse flags to get bundle dir argument so we can open stdin/out/err pipes
				dadooFlags.Parse(cmd.Args[1:])
				processDir := dadooFlags.Arg(2)
				si, so, se, winsz, exit := openPipes(processDir)

				// notify sync pipe - must be before writing stderr
				_, err := fd5.Write([]byte{0})
				Expect(err).NotTo(HaveOccurred())
				Expect(fd5.Close()).To(Succeed())

				// handle window size
				go recvWinSz(winsz)

				// write stderr
				_, err = se.WriteString(stderrContents)
				Expect(err).NotTo(HaveOccurred())

				// write log file to fd4
				_, err = io.Copy(fd4, bytes.NewReader([]byte(logs)))
				Expect(err).NotTo(HaveOccurred())
				fd4.Close()

				if runcHangsForEver {
					for {
						time.Sleep(time.Minute)
					}
				}

				// return exit status of runc on fd3
				if !dadooPanicsBeforeReportingRuncExitCode {
					// if dadooPanics then closes the pipe without writing a value
					_, err = fd3.Write([]byte{runcReturns})
					Expect(err).NotTo(HaveOccurred())
				}
				fd3.Close()
				// write exit code of actual process to $processdir/exitcode file
				if exitCode != nil {
					Eventually(processDir).Should(BeADirectory())
					Expect(ioutil.WriteFile(filepath.Join(processDir, "exitcode"), []byte(exitCode), 0600)).To(Succeed())
				}
				<-closeExitPipeCh
				Expect(exit.Close()).To(Succeed())

				// Sleep before printing any stdout/stderr to allow attach calls to complete
				time.Sleep(time.Millisecond * 50)
				// do some test IO (directly write to stdout and copy stdin->stderr)
				so.WriteString("hello stdout")

				// it's a trap - stdin is copied to stderr so that we can test
				// its content further down
				_, err = io.Copy(se, si)
				Expect(err).NotTo(HaveOccurred())
				se.WriteString("done copying stdin")

				// close streams
				Expect(so.Close()).To(Succeed())
				Expect(se.Close()).To(Succeed())
			}(cmd, runcHangsForEver, dadooPanicsBeforeReportingRuncExitCode, dadooWritesExitCode, []byte(dadooWritesLogs), closeExitPipeCh, receiveWinSize, stderrContents)

			return nil
		})
	})

	Describe("Run", func() {
		Context("when a processID is reused concurrently", func() {
			var processID string
			BeforeEach(func() {
				processID = "same-id"
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a sensible error", func() {
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).To(MatchError(fmt.Sprintf("process ID '%s' already in use", processID)))
			})
		})

		Context("when a processID is reused for a different container ID (different process path)", func() {
			var (
				otherProcessPath string
			)

			BeforeEach(func() {
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				otherProcessPath, err = ioutil.TempDir("", "execrunner-tests")
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{}, bundlePath, otherProcessPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		It("executes the dadoo binary with the correct arguments", func() {
			runner.Run(log, processID, &runrunc.PreparedSpec{}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})

			Expect(fakeCommandRunner.StartedCommands()[0].Args).To(
				ConsistOf(
					"path-to-dadoo",
					"-runc-root", "runc-root",
					"exec", "path-to-runc", filepath.Join(processPath, processID), "some-handle",
				),
			)
		})

		Context("when TTY is requested", func() {
			It("executed the dadoo binary with the correct arguments", func() {
				runner.Run(log, processID, &runrunc.PreparedSpec{
					HostUID: 123,
					HostGID: 456,
				},
					bundlePath, processPath,
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
						"-runc-root", "runc-root",
						"-tty",
						"exec", "path-to-runc", filepath.Join(processPath, processID), "some-handle",
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
				runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				close(runReturns)
			}(runner)

			Eventually(runReturns).Should(BeClosed())

			Expect(fakeCommandRunner.StartedCommands()).To(HaveLen(1))
			Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(0))
			Eventually(fakeCommandRunner.WaitedCommands).Should(ConsistOf(fakeCommandRunner.StartedCommands())) // avoid zombies by waiting
		})

		It("passes the encoded process spec on STDIN of dadoo", func() {
			runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
			Expect(string(receivedStdinContents)).To(ContainSubstring(`"args":["Banana","rama"]`))
			Expect(string(receivedStdinContents)).NotTo(ContainSubstring(`HostUID`))
		})

		Context("when cleanupProcessDirsOnWait is true", func() {
			BeforeEach(func() {
				runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", "runc-root", fakeProcessIDGenerator, fakePidGetter, fakeCommandRunner, true)
			})

			It("cleans up the processes dir after Wait returns", func() {
				process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).To(BeAnExistingFile())

				_, err = process.Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).NotTo(BeAnExistingFile())
			})
		})

		Context("when cleanupProcessDirsOnWait is false", func() {
			BeforeEach(func() {
				runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", "runc-root", fakeProcessIDGenerator, fakePidGetter, fakeCommandRunner, false)
			})

			It("does not clean up the processes dir after Wait returns", func() {
				process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).To(BeAnExistingFile())

				_, err = process.Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).To(BeAnExistingFile())
			})
		})

		Context("when spawning dadoo fails", func() {
			It("returns a nice error", func() {
				dadooReturns = errors.New("boom")

				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).To(MatchError(ContainSubstring("boom")))
			})
		})

		Context("when runc exec fails", func() {
			BeforeEach(func() {
				runcReturns = 3
			})

			Context("when runc writes a lot of data to stderr", func() {
				BeforeEach(func() {
					for i := 0; i < 5000; i++ {
						stderrContents += "I am a bad runC\n"
					}
				})

				It("does not deadlock", func(done Done) {
					_, err := runner.Run(log, processID, &runrunc.PreparedSpec{
						Process: specs.Process{
							Args: []string{"Banana", "rama"},
						},
					}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{
						Stderr: gbytes.NewBuffer(),
					})
					Expect(err).To(MatchError(ContainSubstring("exit status 3")))

					close(done)
				}, 10.0)
			})
		})

		Context("when dadoo panics before reporting runc's exit code", func() {
			BeforeEach(func() {
				dadooPanicsBeforeReportingRuncExitCode = true
			})

			It("returns a meaningful error", func() {
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}},
					bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).To(MatchError(ContainSubstring("failed to read runc exit code")))

			})
		})

		Describe("Logging", func() {
			It("sends all the runc logs to the logger", func() {
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
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

			It("sends all the dadoo logs to the logger after dadoo exits", func() {
				process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				_, err = process.Wait()
				Expect(err).NotTo(HaveOccurred())

				Eventually(log).Should(gbytes.Say("dadoo stdout"))
				Eventually(log).Should(gbytes.Say("dadoo stderr"))
			})

			Context("when `runC exec` fails", func() {
				BeforeEach(func() {
					runcReturns = 3
				})

				It("return an error including the last log line when runC fails to start the container", func() {
					_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).To(MatchError("runc exec: exit status 3: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
				})

				Context("when the log messages can't be parsed", func() {
					BeforeEach(func() {
						dadooWritesLogs = `foobarbaz="'
					`
					})

					It("logs an error including the log line", func() {
						_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).To(HaveOccurred())
						Expect(log.Buffer()).To(gbytes.Say("foobarbaz"))
					})

					It("returns an error with only the exit status if the log can't be parsed", func() {
						_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).To(MatchError(ContainSubstring("runc exec: exit status 3")))
					})
				})
			})

			Context("when runc is slow to exit (or never exits)", func() {
				BeforeEach(func() {
					runcHangsForEver = true
				})

				It("still forwards runc logs in real time", func() {
					go func(log lager.Logger, processID, bundlePath, processPath string) {
						defer GinkgoRecover()
						runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}},
							bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
					}(log, processID, bundlePath, processPath)

					Eventually(func() []lager.LogFormat {
						return log.Logs()
					}).Should(HaveLen(6))

					runcLogs := make([]lager.LogFormat, 0)
					for _, log := range log.Logs() {
						if log.Message == "test.execrunner.runc" {
							runcLogs = append(runcLogs, log)
						}
					}

					Expect(runcLogs).To(HaveLen(3))
					Expect(runcLogs[0].Data).To(HaveKeyWithValue("message", "signal: potato"))
				})
			})
		})

		Describe("the returned garden.Process", func() {
			Context("when an empty process ID is passed", func() {
				BeforeEach(func() {
					fakeProcessIDGenerator.GenerateReturns("some-generated-id")
				})

				It("has a generated ID", func() {
					process, err := runner.Run(log, "", &runrunc.PreparedSpec{}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					Expect(process.ID()).To(Equal("some-generated-id"))
				})
			})

			Context("when a non-empty process ID is passed", func() {
				It("has the passed ID", func() {
					processID := "some-process-id"

					process, err := runner.Run(log, processID, &runrunc.PreparedSpec{}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					Expect(process.ID()).To(Equal(processID))
				})
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

					process, err := runner.Run(log, processID, &runrunc.PreparedSpec{
						HostUID: 123,
						HostGID: 456,
					},
						bundlePath, processPath,
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
					process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					process.Signal(garden.SignalTerminate)
					Expect(fakePidGetter.PidArgsForCall(0)).To(Equal(filepath.Join(processPath, processID, "pidfile")))
				})

				Context("when the pidGetter returns an error", func() {
					BeforeEach(func() {
						fakePidGetter.PidReturns(0, errors.New("Unable to get PID"))
					})

					It("returns an appropriate error", func() {
						process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
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
							processID,
							&runrunc.PreparedSpec{
								Process: specs.Process{
									Args: []string{"echo", "This won't actually do anything as the command runner is faked"},
								},
							},
							bundlePath, processPath,
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
						process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", ""}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
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
						process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
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

					process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).NotTo(HaveOccurred())

					Expect(process.Wait()).To(Equal(42))
				})

				Context("when the exitfile is empty", func() {
					It("returns an error", func() {
						dadooWritesExitCode = []byte("")

						process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).To(MatchError(ContainSubstring("the exitcode file is empty")))
					})
				})

				Context("when the exitfile does not exist", func() {
					It("returns an error", func() {
						dadooWritesExitCode = nil

						process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).To(MatchError(ContainSubstring("could not find the exitcode file for the process")))
					})
				})

				Context("when the exitcode file doesn't contain an exit code", func() {
					It("returns an error", func() {
						dadooWritesExitCode = []byte("potato")

						process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
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
			process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", "ohai"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{
				Stdout: stdout,
				Stderr: stderr,
				Stdin:  strings.NewReader("omg"),
			})
			Expect(err).NotTo(HaveOccurred())

			process.Wait()
			Eventually(stdout).Should(gbytes.Say("hello stdout"))
			Eventually(stderr).Should(gbytes.Say("omg"))
		})

		Context("when running and attaching", func() {
			It("multiplexes stdout and stderr", func() {
				runStdout := gbytes.NewBuffer()
				runStderr := gbytes.NewBuffer()
				process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"sleep4reals"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{
					Stdout: runStdout,
					Stderr: runStderr,
					Stdin:  strings.NewReader("omg"),
				})
				Expect(err).NotTo(HaveOccurred())

				attachStdout := gbytes.NewBuffer()
				attachStderr := gbytes.NewBuffer()
				_, err = runner.Attach(log, processID, garden.ProcessIO{Stdout: attachStdout, Stderr: attachStderr, Stdin: strings.NewReader("")}, processPath)
				Expect(err).NotTo(HaveOccurred())

				process.Wait()
				Eventually(runStdout).Should(gbytes.Say("hello stdout"))
				Eventually(attachStdout).Should(gbytes.Say("hello stdout"))

				Eventually(runStderr).Should(gbytes.Say("omg"))
				Eventually(attachStderr).Should(gbytes.Say("omg"))
			})

			It("cleans up the map entry but not the process path", func() {
				runStdout := gbytes.NewBuffer()
				runStderr := gbytes.NewBuffer()
				process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"sleep4reals"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{
					Stdout: runStdout,
					Stderr: runStderr,
					Stdin:  strings.NewReader("omg"),
				})
				Expect(err).NotTo(HaveOccurred())
				process.Wait()
				Eventually(runStdout).Should(gbytes.Say("hello stdout"))
				Eventually(runStderr).Should(gbytes.Say("omg"))

				Expect(filepath.Join(processPath, processID)).Should(BeADirectory())
				Expect(len(runner.GetProcesses())).To(Equal(0))
			})

			Context("when cleanupProcessDirsOnWait is true", func() {
				JustBeforeEach(func() {
					runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", "runc-root", fakeProcessIDGenerator, fakePidGetter, fakeCommandRunner, true)
				})

				It("cleans up the map entry and the process path", func() {
					runStdout := gbytes.NewBuffer()
					runStderr := gbytes.NewBuffer()
					process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"sleep4reals"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{
						Stdout: runStdout,
						Stderr: runStderr,
						Stdin:  strings.NewReader("omg"),
					})
					Expect(err).NotTo(HaveOccurred())
					process.Wait()
					Eventually(runStdout).Should(gbytes.Say("hello stdout"))
					Eventually(runStderr).Should(gbytes.Say("omg"))

					Expect(filepath.Join(processPath, processID)).ShouldNot(BeADirectory())
					Expect(len(runner.GetProcesses())).To(Equal(0))
				})
			})
		})
		It("closed stdin when the stdin stream ends", func() {
			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()
			process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", "ohai"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{
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
			process, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"echo", "ohai"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{
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

		Context("when cleanupProcessDirsOnWait is true", func() {
			BeforeEach(func() {
				runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", "runc-root", fakeProcessIDGenerator, fakePidGetter, fakeCommandRunner, true)
			})

			It("cleans up the processes dir after Wait returns", func() {
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				process, err := runner.Attach(log, processID, garden.ProcessIO{}, processPath)
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).To(BeAnExistingFile())

				_, err = process.Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).NotTo(BeAnExistingFile())
			})
		})

		Context("when cleanupProcessDirsOnWait is false", func() {
			BeforeEach(func() {
				runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", "runc-root", fakeProcessIDGenerator, fakePidGetter, fakeCommandRunner, false)
			})

			It("does not clean up the processes dir after Wait returns", func() {
				_, err := runner.Run(log, processID, &runrunc.PreparedSpec{Process: specs.Process{Args: []string{"Banana", "rama"}}}, bundlePath, processPath, "some-handle", nil, garden.ProcessIO{})
				process, err := runner.Attach(log, processID, garden.ProcessIO{}, processPath)

				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).To(BeAnExistingFile())

				_, err = process.Wait()
				Expect(err).NotTo(HaveOccurred())

				Expect(filepath.Join(processPath, processID)).To(BeAnExistingFile())
			})
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

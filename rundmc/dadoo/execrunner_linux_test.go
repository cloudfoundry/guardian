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

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/dadoo"
	dadoofakes "github.com/cloudfoundry-incubator/guardian/rundmc/dadoo/dadoofakes"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	fakes "github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/runruncfakes"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Dadoo ExecRunner", func() {
	var (
		fakeIodaemonRunner     *fakes.FakeExecRunner
		fakeCommandRunner      *fake_command_runner.FakeCommandRunner
		fakeProcessIDGenerator *fakes.FakeUidGenerator
		fakePidGetter          *dadoofakes.FakePidGetter
		runner                 *dadoo.ExecRunner
		processPath            string
		pidPath                string
		receivedStdinContents  []byte
		runcReturns            byte
		dadooReturns           error
		dadooWritesLogs        string
		log                    *lagertest.TestLogger
		receiveWinSize         func(winSizeFD *os.File)
	)

	BeforeEach(func() {
		fakeIodaemonRunner = new(fakes.FakeExecRunner)
		fakeCommandRunner = fake_command_runner.New()
		fakeProcessIDGenerator = new(fakes.FakeUidGenerator)
		fakePidGetter = new(dadoofakes.FakePidGetter)

		fakeProcessIDGenerator.GenerateReturns("the-pid")
		fakePidGetter.PidReturns(0, nil)

		bundlePath, err := ioutil.TempDir("", "dadooexecrunnerbundle")
		Expect(err).NotTo(HaveOccurred())
		processPath = filepath.Join(bundlePath, "the-process")
		pidPath = filepath.Join(processPath, "0.pid")

		runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", fakeProcessIDGenerator, fakePidGetter, fakeIodaemonRunner, fakeCommandRunner)
		log = lagertest.NewTestLogger("test")

		runcReturns = 0
		dadooReturns = nil
		dadooWritesLogs = `time="2016-03-02T13:56:38Z" level=warning msg="signal: potato"
				time="2016-03-02T13:56:38Z" level=error msg="fork/exec POTATO: no such file or directory"
				time="2016-03-02T13:56:38Z" level=fatal msg="Container start failed: [10] System error: fork/exec POTATO: no such file or directory"`

		dadooFlags := flag.NewFlagSet("something", flag.PanicOnError)
		dadooFlags.Bool("tty", false, "")
		dadooFlags.Int("uid", 0, "")
		dadooFlags.Int("gid", 0, "")

		receiveWinSize = func(_ *os.File) {}

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

			go receiveWinSize(fd5)

			go func(cmd *exec.Cmd) {
				defer GinkgoRecover()

				// parse flags to get bundle dir argument so we can open stdin/out/err pipes
				dadooFlags.Parse(cmd.Args[1:])
				processDir := dadooFlags.Arg(2)
				si, so, se := openPipes(processDir)

				// write log file to fd4
				_, err = io.Copy(fd4, bytes.NewReader([]byte(dadooWritesLogs)))
				Expect(err).NotTo(HaveOccurred())
				fd4.Close()

				// return exit status on fd3
				_, err = fd3.Write([]byte{runcReturns})
				Expect(err).NotTo(HaveOccurred())
				fd3.Close()

				// do some test IO (directly write to stdout and copy stdin->stderr)
				so.WriteString("hello stdout")
				_, err = io.Copy(se, si)
				Expect(err).NotTo(HaveOccurred())
			}(cmd)

			return dadooReturns
		})
	})

	Describe("Run", func() {
		Describe("Delegating to IODaemonExecRunner", func() {
			Context("when USE_DADOO is not set as an Environment variable", func() {
				It("delegates directly to iodaemon execer", func() {
					runner.Run(log, &runrunc.PreparedSpec{}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(fakeIodaemonRunner.RunCallCount()).To(Equal(1))
				})
			})

			Context("when USE_DADOO is set to true", func() {
				It("does not delegate to iodaemon execer", func() {
					runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(fakeIodaemonRunner.RunCallCount()).To(Equal(0))
				})
			})
		})

		Describe("When dadoo is used to do the exec", func() {
			It("executes the dadoo binary with the correct arguments", func() {
				runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}}}, processPath, "some-handle", nil, garden.ProcessIO{})

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
						Process: specs.Process{
							Env: []string{"USE_DADOO=true"},
						},
					},
						processPath,
						"some-handle",
						&garden.TTYSpec{},
						garden.ProcessIO{},
					)

					Expect(fakeCommandRunner.StartedCommands()[0].Args).To(
						Equal([]string{
							"path-to-dadoo",
							"-tty",
							"-uid", "123",
							"-gid", "456",
							"exec", "path-to-runc", filepath.Join(processPath, "the-pid"), "some-handle",
						}),
					)
				})

				It("sends the initial window size via the winsz pipe", func() {
					var receivedWinSize dadoo.TtySize
					received := make(chan struct{})
					receiveWinSize = func(winSizeFD *os.File) {
						json.NewDecoder(winSizeFD).Decode(&receivedWinSize)
						close(received)
					}

					runner.Run(log, &runrunc.PreparedSpec{
						HostUID: 123,
						HostGID: 456,
						Process: specs.Process{
							Env: []string{"USE_DADOO=true"},
						},
					},
						processPath,
						"some-handle",
						&garden.TTYSpec{
							&garden.WindowSize{
								Columns: 13,
								Rows:    17,
							},
						},
						garden.ProcessIO{},
					)

					Eventually(received).Should(BeClosed())
					Expect(receivedWinSize).To(Equal(dadoo.TtySize{
						Cols: 13,
						Rows: 17,
					}))
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
					runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
					close(runReturns)
				}(runner)

				Eventually(runReturns).Should(BeClosed())

				Expect(fakeCommandRunner.StartedCommands()).To(HaveLen(1))
				Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(0))
				Eventually(fakeCommandRunner.WaitedCommands).Should(ConsistOf(fakeCommandRunner.StartedCommands())) // avoid zombies by waiting
			})

			It("passes the encoded process spec on STDIN of dadoo", func() {
				runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(string(receivedStdinContents)).To(ContainSubstring(`"args":["Banana","rama"]`))
				Expect(string(receivedStdinContents)).NotTo(ContainSubstring(`HostUID`))
			})

			Context("when spawning dadoo fails", func() {
				It("returns a nice error", func() {
					dadooReturns = errors.New("boom")

					_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).To(MatchError(ContainSubstring("boom")))
				})
			})

			Describe("Logging", func() {
				It("sends all the logs to the logger", func() {
					_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
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
						_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).To(MatchError("runc exec: exit status 3: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
					})

					Context("when the log messages can't be parsed", func() {
						BeforeEach(func() {
							dadooWritesLogs = `foo="'
					`
						})

						It("returns an error with only the exit status if the log can't be parsed", func() {
							_, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
							Expect(err).To(MatchError("runc exec: exit status 3: "))
						})
					})
				})
			})

			Describe("the returned garden.Process", func() {
				Describe("SetTTY", func() {
					It("sends the new window size via the winsz pipe", func() {
						var receivedWinSize dadoo.TtySize
						received := make(chan bool)
						receiveWinSize = func(winSizeFD *os.File) {
							json.NewDecoder(winSizeFD).Decode(&receivedWinSize)
							received <- true
							json.NewDecoder(winSizeFD).Decode(&receivedWinSize)
							received <- true
						}

						process, err := runner.Run(log, &runrunc.PreparedSpec{
							HostUID: 123,
							HostGID: 456,
							Process: specs.Process{
								Env: []string{"USE_DADOO=true"},
							},
						},
							processPath,
							"some-handle",
							&garden.TTYSpec{
								&garden.WindowSize{
									Columns: 13,
									Rows:    17,
								},
							},
							garden.ProcessIO{},
						)
						Expect(err).NotTo(HaveOccurred())

						Eventually(received).Should(Receive())
						process.SetTTY(garden.TTYSpec{&garden.WindowSize{Columns: 53, Rows: 59}})

						Eventually(received, "5s").Should(Receive())
						Expect(receivedWinSize).To(Equal(dadoo.TtySize{
							Cols: 53,
							Rows: 59,
						}))

					})
				})

				Describe("Signal", func() {
					It("reads the PID from the pid file", func() {
						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						process.Signal(garden.SignalTerminate)
						Expect(fakePidGetter.PidArgsForCall(0)).To(Equal(filepath.Join(processPath, "the-pid", "pidfile")))
					})

					Context("when the pidGetter returns an error", func() {
						BeforeEach(func() {
							fakePidGetter.PidReturns(0, errors.New("Unable to get PID"))
						})

						It("returns an appropriate error", func() {
							process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
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
										Env:  []string{"USE_DADOO=true"},
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
							process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"echo", ""}}}, processPath, "some-handle", nil, garden.ProcessIO{})
							Expect(err).NotTo(HaveOccurred())

							Expect(process.Signal(garden.SignalTerminate)).To(MatchError("os: process not initialized"))
						})
					})
				})

				Describe("Wait", func() {
					It("returns the exit code of the dadoo process", func() {
						fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: "path-to-dadoo"}, func(cmd *exec.Cmd) error {
							return fakeExitError(42)
						})

						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						Expect(process.Wait()).To(Equal(42))
					})

					It("only calls process.Wait once", func() {
						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).NotTo(HaveOccurred())

						Consistently(fakeCommandRunner.WaitedCommands).Should(HaveLen(1))
					})

					It("returns error if waiting on dadoo fails for a reason other than a regular exit error", func() {
						fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: "path-to-dadoo"}, func(cmd *exec.Cmd) error {
							return errors.New("not ok")
						})

						process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).To(MatchError("not ok"))
					})
				})
			})

			It("can get stdout/err from the spawned process via named pipes", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()
				process, err := runner.Run(log, &runrunc.PreparedSpec{Process: specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"echo", "ohai"}}}, processPath, "some-handle", nil, garden.ProcessIO{
					Stdout: stdout,
					Stderr: stderr,
					Stdin:  strings.NewReader("omg"),
				})
				Expect(err).NotTo(HaveOccurred())

				process.Wait()
				Eventually(stdout).Should(gbytes.Say("hello stdout"))
				Eventually(stderr).Should(gbytes.Say("omg"))
			})
		})
	})

	Describe("Attach", func() {
		It("delegated directly to iodaemon execer", func() {
			runner.Attach(log, "some-process-id", garden.ProcessIO{}, processPath)

			Expect(fakeIodaemonRunner.AttachCallCount()).To(Equal(1))
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

func openPipes(dir string) (stdin, stdout, stderr *os.File) {
	si, err := os.Open(filepath.Join(dir, "stdin"))
	Expect(err).NotTo(HaveOccurred())

	so, err := os.OpenFile(filepath.Join(dir, "stdout"), os.O_APPEND|os.O_WRONLY, 0600)
	Expect(err).NotTo(HaveOccurred())

	se, err := os.OpenFile(filepath.Join(dir, "stderr"), os.O_APPEND|os.O_WRONLY, 0600)
	Expect(err).NotTo(HaveOccurred())

	return si, so, se
}

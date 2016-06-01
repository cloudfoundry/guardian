package dadoo_test

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/dadoo"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc/fakes"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Dadoo ExecRunner", func() {
	var (
		fakeIodaemonRunner    *fakes.FakeExecRunner
		fakeCommandRunner     *fake_command_runner.FakeCommandRunner
		fakePidGenerator      *fakes.FakeUidGenerator
		runner                *dadoo.ExecRunner
		processPath           string
		receivedStdinContents []byte
		dadooReturns          error
		log                   lager.Logger
	)

	BeforeEach(func() {
		fakeIodaemonRunner = new(fakes.FakeExecRunner)
		fakeCommandRunner = fake_command_runner.New()
		fakePidGenerator = new(fakes.FakeUidGenerator)

		fakePidGenerator.GenerateReturns("the-pid")

		bundlePath, err := ioutil.TempDir("", "dadooexecrunnerbundle")
		Expect(err).NotTo(HaveOccurred())
		processPath = filepath.Join(bundlePath, "the-process")

		runner = dadoo.NewExecRunner("path-to-dadoo", "path-to-runc", fakePidGenerator, fakeIodaemonRunner, fakeCommandRunner)
		log = lagertest.NewTestLogger("test")

		dadooReturns = nil

		// dadoo should open up its end of the named pipes
		fakeCommandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "path-to-dadoo",
		}, func(cmd *exec.Cmd) error {
			var err error
			receivedStdinContents, err = ioutil.ReadAll(cmd.Stdin)
			Expect(err).NotTo(HaveOccurred())

			go func(cmd *exec.Cmd) {
				defer GinkgoRecover()

				fs := flag.NewFlagSet("something", flag.PanicOnError)
				stdin := fs.String("stdin", "", "")
				stdout := fs.String("stdout", "", "")
				stderr := fs.String("stderr", "", "")
				fs.String("waitSock", "", "")
				fs.Parse(cmd.Args[1:])

				si, err := os.Open(*stdin)
				Expect(err).NotTo(HaveOccurred())

				so, err := os.OpenFile(*stdout, os.O_APPEND|os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

				se, err := os.OpenFile(*stderr, os.O_APPEND|os.O_WRONLY, 0600)
				Expect(err).NotTo(HaveOccurred())

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
					runner.Run(log, &specs.Process{}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(fakeIodaemonRunner.RunCallCount()).To(Equal(1))
				})
			})

			Context("when USE_DADOO is set to true", func() {
				It("does not delegate to iodaemon execer", func() {
					runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(fakeIodaemonRunner.RunCallCount()).To(Equal(0))
				})
			})
		})

		Describe("When dadoo is used to do the exec", func() {
			It("executes the dadoo binary with the correct arguments", func() {
				runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}}, processPath, "some-handle", nil, garden.ProcessIO{})

				Expect(fakeCommandRunner.StartedCommands()[0].Args).To(
					ConsistOf(
						"path-to-dadoo",
						"-stdin", filepath.Join(processPath, "the-pid", "stdin"),
						"-stdout", filepath.Join(processPath, "the-pid", "stdout"),
						"-stderr", filepath.Join(processPath, "the-pid", "stderr"),
						"exec", "path-to-runc", filepath.Join(processPath, "the-pid"), "some-handle",
					),
				)
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
					runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}, processPath, "some-handle", nil, garden.ProcessIO{})
					close(runReturns)
				}(runner)

				Eventually(runReturns).Should(BeClosed())

				Expect(fakeCommandRunner.StartedCommands()).To(HaveLen(1))
				Expect(fakeCommandRunner.ExecutedCommands()).To(HaveLen(0))
				Eventually(fakeCommandRunner.WaitedCommands).Should(ConsistOf(fakeCommandRunner.StartedCommands())) // avoid zombies by waiting
			})

			It("passes the encoded process spec on STDIN of dadoo", func() {
				runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}, processPath, "some-handle", nil, garden.ProcessIO{})
				Expect(string(receivedStdinContents)).To(ContainSubstring(`"args":["Banana","rama"]`))
			})

			Context("when spawning dadoo fails", func() {
				It("returns a nice error", func() {
					dadooReturns = errors.New("boom")

					_, err := runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}, processPath, "some-handle", nil, garden.ProcessIO{})
					Expect(err).To(MatchError(ContainSubstring("boom")))
				})
			})

			Describe("the returned garden.Process", func() {
				Describe("Wait", func() {
					It("returns the exit code of the dadoo process", func() {
						fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: "path-to-dadoo"}, func(cmd *exec.Cmd) error {
							return fakeExitError(42)
						})

						process, err := runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						Expect(process.Wait()).To(Equal(42))
					})

					It("only calls process.Wait once", func() {
						process, err := runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).NotTo(HaveOccurred())

						Consistently(fakeCommandRunner.WaitedCommands).Should(HaveLen(1))
					})

					It("returns error if waiting on dadoo fails for a reason other than a regular exit error", func() {
						fakeCommandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{Path: "path-to-dadoo"}, func(cmd *exec.Cmd) error {
							return errors.New("not ok")
						})

						process, err := runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"Banana", "rama"}}, processPath, "some-handle", nil, garden.ProcessIO{})
						Expect(err).NotTo(HaveOccurred())

						_, err = process.Wait()
						Expect(err).To(MatchError("not ok"))
					})
				})
			})

			It("can get stdout/err from the spawned process via named pipes", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()
				process, err := runner.Run(log, &specs.Process{Env: []string{"USE_DADOO=true"}, Args: []string{"echo", "ohai"}}, processPath, "some-handle", nil, garden.ProcessIO{
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

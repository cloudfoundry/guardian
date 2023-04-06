package runrunc_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Create", func() {
	var (
		commandRunner  *fake_command_runner.FakeCommandRunner
		eventsWatcher  *runruncfakes.FakeEventsWatcher
		fakeDepot      *runruncfakes.FakeDepot
		bundlePath     string
		runcExtraArgs  = []string{"--some-arg", "some-value"}
		logFilePath    string
		pidFilePath    string
		logger         *lagertest.TestLogger
		logs           string
		runcExitStatus error
		recievedStdin  string

		runner *runrunc.Creator
	)

	BeforeEach(func() {
		logs = ""
		runcExitStatus = nil
		commandRunner = fake_command_runner.New()
		eventsWatcher = new(runruncfakes.FakeEventsWatcher)
		fakeDepot = new(runruncfakes.FakeDepot)
		logger = lagertest.NewTestLogger("test")

		var err error
		bundlePath, err = ioutil.TempDir("", "bundle-path")
		Expect(err).NotTo(HaveOccurred())
		fakeDepot.CreateReturns(bundlePath, nil)
		logFilePath = filepath.Join(bundlePath, "create.log")
		pidFilePath = filepath.Join(bundlePath, "pidfile")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		runner = runrunc.NewCreator(goci.RuncBinary{Path: "funC"}, runcExtraArgs, commandRunner, eventsWatcher, fakeDepot)

		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "funC",
		}, func(cmd *exec.Cmd) error {
			logFile, err := os.Create(cmd.Args[3])
			Expect(err).NotTo(HaveOccurred())
			_, err = io.Copy(logFile, strings.NewReader(logs))
			Expect(err).NotTo(HaveOccurred())

			if cmd.Stdin != nil {
				stdinBytes, err := ioutil.ReadAll(cmd.Stdin)
				Expect(err).NotTo(HaveOccurred())
				recievedStdin = string(stdinBytes)
			}

			Expect(logFile.Close()).To(Succeed())
			return runcExitStatus
		})
	})

	It("creates the container with runC subcommand", func() {
		Expect(runner.Create(logger, "some-id", goci.Bndl{}, garden.ProcessIO{})).To(Succeed())

		Expect(commandRunner.ExecutedCommands()[0].Path).To(Equal("funC"))
		Expect(commandRunner.ExecutedCommands()[0].Args).To(ConsistOf(
			"funC",
			"--debug",
			"--log", logFilePath,
			"--log-format", "json",
			runcExtraArgs[0], runcExtraArgs[1],
			"run",
			"--detach",
			"--no-new-keyring",
			"--bundle", bundlePath,
			"--pid-file", pidFilePath,
			"some-id",
		))
	})

	It("creates a bundle in the depot", func() {
		bundle := goci.Bndl{Spec: specs.Spec{Version: "version"}}
		Expect(runner.Create(logger, "some-id", bundle, garden.ProcessIO{})).To(Succeed())

		Expect(fakeDepot.CreateCallCount()).To(Equal(1))
		_, actualID, actualBundle := fakeDepot.CreateArgsForCall(0)
		Expect(actualID).To(Equal("some-id"))
		Expect(actualBundle).To(Equal(bundle))
	})

	It("attaches the stdout and stderr directly to the runC command", func() {
		pio := garden.ProcessIO{
			Stdin:  nil,
			Stdout: bytes.NewBufferString("some-stdout"),
			Stderr: bytes.NewBufferString("some-stderr"),
		}
		Expect(runner.Create(logger, "some-id", goci.Bndl{}, pio)).To(Succeed())
		Expect(commandRunner.ExecutedCommands()[0].Stdout).To(Equal(pio.Stdout))
		Expect(commandRunner.ExecutedCommands()[0].Stderr).To(Equal(pio.Stderr))
	})

	It("shuttles the stdin to the runC command", func() {
		pio := garden.ProcessIO{
			Stdin: bytes.NewBufferString("some-stdin"),
		}
		Expect(runner.Create(logger, "some-id", goci.Bndl{}, pio)).To(Succeed())
		Expect(recievedStdin).To(Equal("some-stdin"))
	})

	It("subscribes for container events", func() {
		Expect(runner.Create(logger, "some-id", goci.Bndl{}, garden.ProcessIO{})).To(Succeed())
		Eventually(eventsWatcher.WatchEventsCallCount).Should(Equal(1))
		_, actualHandle := eventsWatcher.WatchEventsArgsForCall(0)
		Expect(actualHandle).To(Equal("some-id"))
	})

	Context("when creating in the depot fails", func() {
		BeforeEach(func() {
			fakeDepot.CreateReturns("", errors.New("musaka"))
		})

		It("propagates the errors", func() {
			Expect(runner.Create(logger, "some-id", goci.Bndl{}, garden.ProcessIO{})).To(MatchError("musaka"))
		})
	})

	Context("when running runc fails", func() {
		BeforeEach(func() {
			runcExitStatus = errors.New("some-error")
		})

		It("returns runc's exit status", func() {
			Expect(runner.Create(logger, "some-id", goci.Bndl{}, garden.ProcessIO{})).To(MatchError("runc run: some-error: "))
		})

		It("does not subscribe for container events", func() {
			Consistently(eventsWatcher.WatchEventsCallCount()).Should(BeZero())
		})
	})

	Describe("forwarding logs from runC", func() {
		BeforeEach(func() {
			logs = `{"time":"2016-03-02T13:56:38Z", "level":"warning", "msg":"signal: potato"}
{"time":"2016-03-02T13:56:38Z", "level":"error", "msg":"fork/exec POTATO: no such file or directory"}
{"time":"2016-03-02T13:56:38Z", "level":"fatal", "msg":"Container start failed: [10] System error: fork/exec POTATO: no such file or directory"}`
		})

		It("sends all the logs to the logger", func() {
			Expect(runner.Create(logger, "some-id", goci.Bndl{}, garden.ProcessIO{})).To(Succeed())

			runcLogs := make([]lager.LogFormat, 0)
			for _, log := range logger.Logs() {
				if log.Message == "test.create.runc" {
					runcLogs = append(runcLogs, log)
				}
			}

			Expect(runcLogs).To(HaveLen(3))
			Expect(runcLogs[0].Data).To(HaveKeyWithValue("message", "signal: potato"))
		})

		Context("when runC fails", func() {
			BeforeEach(func() {
				runcExitStatus = errors.New("boom")
			})

			It("return an error including parsed logs when runC fails to start the container", func() {
				Expect(runner.Create(logger, "some-id", goci.Bndl{}, garden.ProcessIO{})).To(MatchError("runc run: boom: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
			})
		})
	})
})

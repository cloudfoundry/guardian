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
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	var (
		commandRunner  *fake_command_runner.FakeCommandRunner
		bundlePath     string
		runcSubcmd     = "do-a-thing"
		logFilePath    string
		pidFilePath    string
		logger         *lagertest.TestLogger
		logs           string
		runcExitStatus error

		runner *runrunc.Creator
	)

	BeforeEach(func() {
		logs = ""
		runcExitStatus = nil
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		var err error
		bundlePath, err = ioutil.TempDir("", "bundle")
		Expect(err).NotTo(HaveOccurred())
		logFilePath = filepath.Join(bundlePath, "create.log")
		pidFilePath = filepath.Join(bundlePath, "pidfile")
	})

	JustBeforeEach(func() {
		runner = runrunc.NewCreator("funC", runcSubcmd, commandRunner)

		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "funC",
		}, func(cmd *exec.Cmd) error {
			logFile, err := os.Create(cmd.Args[3])
			Expect(err).NotTo(HaveOccurred())
			_, err = io.Copy(logFile, strings.NewReader(logs))
			Expect(err).NotTo(HaveOccurred())
			Expect(logFile.Close()).To(Succeed())
			return runcExitStatus
		})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	It("creates the container with runC subcommand", func() {
		Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())

		Expect(commandRunner.ExecutedCommands()[0].Path).To(Equal("funC"))
		Expect(commandRunner.ExecutedCommands()[0].Args).To(ConsistOf(
			"funC",
			"--debug",
			"--log", logFilePath,
			"--log-format", "json",
			runcSubcmd,
			"--no-new-keyring",
			"--bundle", bundlePath,
			"--pid-file", pidFilePath,
			"some-id",
		))
	})

	It("attaches the processIO to the runC command", func() {
		pio := garden.ProcessIO{
			Stdin:  strings.NewReader("some-stdin"),
			Stdout: bytes.NewBufferString("some-stdout"),
			Stderr: bytes.NewBufferString("some-stderr"),
		}
		Expect(runner.Create(logger, bundlePath, "some-id", pio)).To(Succeed())
		Expect(commandRunner.ExecutedCommands()[0].Stdin).To(Equal(pio.Stdin))
		Expect(commandRunner.ExecutedCommands()[0].Stdout).To(Equal(pio.Stdout))
		Expect(commandRunner.ExecutedCommands()[0].Stderr).To(Equal(pio.Stderr))
	})

	Context("when running runc fails", func() {
		BeforeEach(func() {
			runcExitStatus = errors.New("some-error")
		})

		It("returns runc's exit status", func() {
			Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc do-a-thing: some-error: "))
		})
	})

	Describe("forwarding logs from runC", func() {
		BeforeEach(func() {
			logs = `{"time":"2016-03-02T13:56:38Z", "level":"warning", "msg":"signal: potato"}
{"time":"2016-03-02T13:56:38Z", "level":"error", "msg":"fork/exec POTATO: no such file or directory"}
{"time":"2016-03-02T13:56:38Z", "level":"fatal", "msg":"Container start failed: [10] System error: fork/exec POTATO: no such file or directory"}`
		})

		It("sends all the logs to the logger", func() {
			Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())

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
				Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc do-a-thing: boom: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
			})

			Context("when the log messages can't be parsed", func() {
				BeforeEach(func() {
					logs = "garbage\n"
				})

				It("returns an error with the last non-empty line", func() {
					Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc do-a-thing: boom: garbage"))
				})
			})
		})
	})
})

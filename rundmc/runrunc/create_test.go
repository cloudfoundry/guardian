package runrunc_test

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Create", func() {
	var (
		commandRunner  *fake_command_runner.FakeCommandRunner
		bundlePath     string
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

		runner = runrunc.NewCreator("funC", commandRunner)
	})

	JustBeforeEach(func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "funC",
		}, func(cmd *exec.Cmd) error {
			logFile, err := os.Create(cmd.Args[3])
			Expect(err).NotTo(HaveOccurred())
			_, err = io.Copy(logFile, bytes.NewReader([]byte(logs)))
			Expect(err).NotTo(HaveOccurred())
			return runcExitStatus
		})
	})

	It("creates the container with runC create", func() {
		Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())

		Expect(commandRunner.ExecutedCommands()[0].Path).To(Equal("funC"))
		Expect(commandRunner.ExecutedCommands()[0].Args).To(ConsistOf(
			"funC", "--debug", "--log", logFilePath, "create", "--no-new-keyring", "--bundle", bundlePath, "--pid-file", pidFilePath, "some-id",
		))
	})

	Context("when running runc fails", func() {
		BeforeEach(func() {
			runcExitStatus = errors.New("some-error")
		})

		It("returns runc's exit status", func() {
			Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc create: some-error: "))
		})
	})

	Describe("forwarding logs from runC", func() {

		BeforeEach(func() {
			logs = `time="2016-03-02T13:56:38Z" level=warning msg="signal: potato"
				time="2016-03-02T13:56:38Z" level=error msg="fork/exec POTATO: no such file or directory"
				time="2016-03-02T13:56:38Z" level=fatal msg="Container start failed: [10] System error: fork/exec POTATO: no such file or directory"`
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

		Context("when `runC create` fails", func() {
			BeforeEach(func() {
				runcExitStatus = errors.New("boom")
			})

			It("return an error including parsed logs when runC fails to start the container", func() {
				Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc create: boom: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
			})

			Context("when the log messages can't be parsed", func() {
				BeforeEach(func() {
					logs = `foo="'
					`
				})

				It("returns an error with only the exit status", func() {
					Expect(runner.Create(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc create: boom: "))
				})
			})
		})
	})
})

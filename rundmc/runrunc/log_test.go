package runrunc_test

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"

	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunAndLog", func() {
	const logs string = `time="2016-03-02T13:56:38Z" level=warning msg="signal: potato"
				time="2016-03-02T13:56:38Z" level=error msg="fork/exec POTATO: no such file or directory"
				time="2016-03-02T13:56:38Z" level=fatal msg="Container start failed: [10] System error: fork/exec POTATO: no such file or directory"`

	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		logRunner     runrunc.RuncCmdRunner
		logger        *lagertest.TestLogger

		logFile *os.File
	)

	BeforeEach(func() {
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		var err error
		logFile, err = ioutil.TempFile("", "runandlog")
		Expect(err).NotTo(HaveOccurred())

		logRunner = runrunc.NewLogRunner(commandRunner, func() (*os.File, error) {
			return logFile, nil
		})
	})

	It("execs the command", func() {
		Expect(logRunner.RunAndLog(logger, func(logFile string) *exec.Cmd {
			return exec.Command("something.exe")
		})).To(Succeed())

		Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
			Path: "something.exe",
		}))
	})

	It("forwards any logs coming from the log file", func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "something.exe",
		}, func(cmd *exec.Cmd) error {
			ioutil.WriteFile(cmd.Args[1], []byte(logs), 0777)
			return nil
		})

		logRunner.RunAndLog(logger, func(logFile string) *exec.Cmd {
			return exec.Command("something.exe", logFile)
		})

		runcLogs := make([]lager.LogFormat, 0)
		for _, log := range logger.Logs() {
			if log.Message == "test.run.runc" {
				runcLogs = append(runcLogs, log)
			}
		}

		Expect(runcLogs).To(HaveLen(3))
		Expect(runcLogs[0].Data).To(HaveKeyWithValue("message", "signal: potato"))
	})

	It("wraps errors returned by run with the last log message", func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "something.exe",
		}, func(cmd *exec.Cmd) error {
			ioutil.WriteFile(cmd.Args[1], []byte(logs), 0777)
			return errors.New("potato")
		})

		Expect(logRunner.RunAndLog(logger, func(logFile string) *exec.Cmd {
			return exec.Command("something.exe", logFile)
		})).To(MatchError(MatchRegexp("potato: .*System error.*POTATO.*")))
	})

	It("deletes the log file when it's done", func() {
		var logFileName string

		err := logRunner.RunAndLog(logger, func(logFile string) *exec.Cmd {
			logFileName = logFile
			Expect(logFile).To(BeARegularFile())
			return exec.Command("something.exe", logFile)
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(logFileName).NotTo(BeARegularFile())
	})

	Describe("Log File Generator", func() {
		It("generates unique log files", func() {
			a, err := runrunc.LogDir("").GenerateLogFile()
			Expect(err).NotTo(HaveOccurred())

			b, err := runrunc.LogDir("").GenerateLogFile()
			Expect(err).NotTo(HaveOccurred())

			Expect(a.Name()).NotTo(Equal(b.Name()))
			Expect(os.Remove(a.Name())).To(Succeed())
			Expect(os.Remove(b.Name())).To(Succeed())
		})

		It("generates files within a given directory", func() {
			dir, err := ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			b, err := runrunc.LogDir(dir).GenerateLogFile()
			Expect(err).NotTo(HaveOccurred())

			Expect(b.Name()).To(HavePrefix(dir))
			Expect(os.RemoveAll(dir)).To(Succeed())
		})
	})
})

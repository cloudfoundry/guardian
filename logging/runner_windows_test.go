package logging_test

import (
	"bytes"
	"os/exec"
	"time"

	"code.cloudfoundry.org/commandrunner"
	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	"code.cloudfoundry.org/commandrunner/windows_command_runner"
	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logging Runner", func() {
	var innerRunner commandrunner.CommandRunner
	var logger *lagertest.TestLogger

	var runner *logging.Runner

	BeforeEach(func() {
		innerRunner = windows_command_runner.New(true)
		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		runner = &logging.Runner{
			CommandRunner: innerRunner,
			Logger:        logger,
		}
	})

	It("logs the duration it took to run the command", func() {
		err := runner.Run(exec.Command("powershell.exe", "-Command", "Start-Sleep", "1"))
		Expect(err).ToNot(HaveOccurred())

		Expect(logger.TestSink.Logs()).To(HaveLen(2))

		log := logger.TestSink.Logs()[1]

		took := log.Data["took"].(string)
		Expect(took).ToNot(BeEmpty())

		duration, err := time.ParseDuration(took)
		Expect(err).ToNot(HaveOccurred())
		Expect(duration).To(BeNumerically(">=", 1*time.Second))
	})

	It("logs the command's argv", func() {
		err := runner.Run(exec.Command("powershell.exe", "-Command", "Write-Host sup"))
		Expect(err).ToNot(HaveOccurred())

		Expect(logger.TestSink.Logs()).To(HaveLen(2))

		log := logger.TestSink.Logs()[0]
		Expect(log.LogLevel).To(Equal(lager.DEBUG))
		Expect(log.Message).To(Equal("test.command.starting"))
		Expect(log.Data["argv"]).To(Equal([]interface{}{"powershell.exe", "-Command", "Write-Host sup"}))

		log = logger.TestSink.Logs()[1]
		Expect(log.LogLevel).To(Equal(lager.DEBUG))
		Expect(log.Message).To(Equal("test.command.succeeded"))
		Expect(log.Data["argv"]).To(Equal([]interface{}{"powershell.exe", "-Command", "Write-Host sup"}))
	})

	Describe("running a command that exits normally", func() {
		It("logs its exit status with 'debug' level", func() {
			err := runner.Run(exec.Command("cmd.exe", "/c", "dir"))
			Expect(err).ToNot(HaveOccurred())

			Expect(logger.TestSink.Logs()).To(HaveLen(2))

			log := logger.TestSink.Logs()[1]
			Expect(log.LogLevel).To(Equal(lager.DEBUG))
			Expect(log.Message).To(Equal("test.command.succeeded"))
			Expect(log.Data["exit-status"]).To(Equal(float64(0)))
		})

		Context("when the command has output to stdout/stderr", func() {
			It("does not log stdout/stderr", func() {
				err := runner.Run(exec.Command("powershell.exe", "-Command", "Write-Host 'hi out'; $host.ui.WriteErrorLine('hi err')"))
				Expect(err).ToNot(HaveOccurred())

				Expect(logger.TestSink.Logs()).To(HaveLen(2))

				log := logger.TestSink.Logs()[1]
				Expect(log.LogLevel).To(Equal(lager.DEBUG))
				Expect(log.Message).To(Equal("test.command.succeeded"))
				Expect(log.Data).ToNot(HaveKey("stdout"))
				Expect(log.Data).ToNot(HaveKey("stderr"))
			})
		})
	})

	Describe("delegation", func() {
		var fakeRunner *fake_command_runner.FakeCommandRunner

		BeforeEach(func() {
			fakeRunner = fake_command_runner.New()
			innerRunner = fakeRunner
		})

		It("runs using the provided runner", func() {
			err := runner.Run(exec.Command("morgan-freeman"))
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "morgan-freeman",
			}))
		})
	})

	Describe("running a bogus command", func() {
		It("logs the error", func() {
			err := runner.Run(exec.Command("morgan-freeman"))
			Expect(err).To(HaveOccurred())

			Expect(logger.TestSink.Logs()).To(HaveLen(2))

			log := logger.TestSink.Logs()[1]
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Message).To(Equal("test.command.failed"))
			Expect(log.Data["error"]).ToNot(BeEmpty())
			Expect(log.Data).ToNot(HaveKey("exit-status"))
		})
	})

	Describe("running a command that exits nonzero", func() {
		It("logs its status with 'error' level", func() {
			err := runner.Run(exec.Command("powershell.exe", "-Command", "Exit 1"))
			Expect(err).To(HaveOccurred())

			Expect(logger.TestSink.Logs()).To(HaveLen(2))

			log := logger.TestSink.Logs()[1]
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Message).To(Equal("test.command.failed"))
			Expect(log.Data["error"]).To(Equal("exit status 1"))
			Expect(log.Data["exit-status"]).To(Equal(float64(1)))
		})

		Context("when the command has output to stdout/stderr", func() {
			It("reports the stdout/stderr in the log data", func() {
				err := runner.Run(exec.Command("powershell.exe", "-Command", "Write-Host 'hi out'; $host.ui.WriteErrorLine('hi err'); Exit 1"))
				Expect(err).To(HaveOccurred())

				Expect(logger.TestSink.Logs()).To(HaveLen(2))

				log := logger.TestSink.Logs()[1]
				Expect(log.LogLevel).To(Equal(lager.ERROR))
				Expect(log.Message).To(Equal("test.command.failed"))
				Expect(log.Data["stdout"]).To(ContainSubstring("hi out"))
				Expect(log.Data["stderr"]).To(ContainSubstring("hi err"))
			})

			Context("and it is being collected by the caller", func() {
				It("multiplexes to the caller and the logs", func() {
					stdout := new(bytes.Buffer)
					stderr := new(bytes.Buffer)

					cmd := exec.Command("powershell.exe", "-Command", "Write-Host 'hi out'; $host.ui.WriteErrorLine('hi err'); Exit 1")
					cmd.Stdout = stdout
					cmd.Stderr = stderr

					err := runner.Run(cmd)
					Expect(err).To(HaveOccurred())

					Expect(logger.TestSink.Logs()).To(HaveLen(2))

					log := logger.TestSink.Logs()[1]
					Expect(log.LogLevel).To(Equal(lager.ERROR))
					Expect(log.Message).To(Equal("test.command.failed"))
					Expect(log.Data["stdout"]).To(ContainSubstring("hi out"))
					Expect(log.Data["stderr"]).To(ContainSubstring("hi err"))

					Expect(stdout.String()).To(ContainSubstring("hi out"))
					Expect(stderr.String()).To(ContainSubstring("hi err"))
				})
			})
		})
	})
})

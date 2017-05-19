package logging_test

import (
	"bytes"
	"os/exec"
	"time"

	"code.cloudfoundry.org/guardian/logging"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/gunk/command_runner"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
	"github.com/cloudfoundry/gunk/command_runner/linux_command_runner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logging Runner", func() {
	var innerRunner command_runner.CommandRunner
	var logger *lagertest.TestLogger

	var runner *logging.Runner

	BeforeEach(func() {
		innerRunner = linux_command_runner.New()
		logger = lagertest.NewTestLogger("test")
	})

	JustBeforeEach(func() {
		runner = &logging.Runner{
			CommandRunner: innerRunner,
			Logger:        logger,
		}
	})

	It("logs the duration it took to run the command", func() {
		err := runner.Run(exec.Command("sleep", "1"))
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
		err := runner.Run(exec.Command("bash", "-c", "echo sup"))
		Expect(err).ToNot(HaveOccurred())

		Expect(logger.TestSink.Logs()).To(HaveLen(2))

		log := logger.TestSink.Logs()[0]
		Expect(log.LogLevel).To(Equal(lager.DEBUG))
		Expect(log.Message).To(Equal("test.command.starting"))
		Expect(log.Data["argv"]).To(Equal([]interface{}{"bash", "-c", "echo sup"}))

		log = logger.TestSink.Logs()[1]
		Expect(log.LogLevel).To(Equal(lager.DEBUG))
		Expect(log.Message).To(Equal("test.command.succeeded"))
		Expect(log.Data["argv"]).To(Equal([]interface{}{"bash", "-c", "echo sup"}))
	})

	Describe("running a command that exits normally", func() {
		It("logs its exit status with 'debug' level", func() {
			err := runner.Run(exec.Command("true"))
			Expect(err).ToNot(HaveOccurred())

			Expect(logger.TestSink.Logs()).To(HaveLen(2))

			log := logger.TestSink.Logs()[1]
			Expect(log.LogLevel).To(Equal(lager.DEBUG))
			Expect(log.Message).To(Equal("test.command.succeeded"))
			Expect(log.Data["exit-status"]).To(Equal(float64(0))) // JSOOOOOOOOOOOOOOOOOOON
		})

		Context("when the command has output to stdout/stderr", func() {
			It("does not log stdout/stderr", func() {
				err := runner.Run(exec.Command("sh", "-c", "echo hi out; echo hi err >&2"))
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
			err := runner.Run(exec.Command("false"))
			Expect(err).To(HaveOccurred())

			Expect(logger.TestSink.Logs()).To(HaveLen(2))

			log := logger.TestSink.Logs()[1]
			Expect(log.LogLevel).To(Equal(lager.ERROR))
			Expect(log.Message).To(Equal("test.command.failed"))
			Expect(log.Data["error"]).To(Equal("exit status 1"))
			Expect(log.Data["exit-status"]).To(Equal(float64(1))) // JSOOOOOOOOOOOOOOOOOOON
		})

		Context("when the command has output to stdout/stderr", func() {
			It("reports the stdout/stderr in the log data", func() {
				err := runner.Run(exec.Command("sh", "-c", "echo hi out; echo hi err >&2; exit 1"))
				Expect(err).To(HaveOccurred())

				Expect(logger.TestSink.Logs()).To(HaveLen(2))

				log := logger.TestSink.Logs()[1]
				Expect(log.LogLevel).To(Equal(lager.ERROR))
				Expect(log.Message).To(Equal("test.command.failed"))
				Expect(log.Data["stdout"]).To(Equal("hi out\n"))
				Expect(log.Data["stderr"]).To(Equal("hi err\n"))
			})

			Context("and it is being collected by the caller", func() {
				It("multiplexes to the caller and the logs", func() {
					stdout := new(bytes.Buffer)
					stderr := new(bytes.Buffer)

					cmd := exec.Command("sh", "-c", "echo hi out; echo hi err >&2; exit 1")
					cmd.Stdout = stdout
					cmd.Stderr = stderr

					err := runner.Run(cmd)
					Expect(err).To(HaveOccurred())

					Expect(logger.TestSink.Logs()).To(HaveLen(2))

					log := logger.TestSink.Logs()[1]
					Expect(log.LogLevel).To(Equal(lager.ERROR))
					Expect(log.Message).To(Equal("test.command.failed"))
					Expect(log.Data["stdout"]).To(Equal("hi out\n"))
					Expect(log.Data["stderr"]).To(Equal("hi err\n"))

					Expect(stdout.String()).To(Equal("hi out\n"))
					Expect(stderr.String()).To(Equal("hi err\n"))
				})
			})
		})
	})
})

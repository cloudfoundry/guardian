package systemreporter_test

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	"code.cloudfoundry.org/grootfs/metrics/systemreporter"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LogBased", func() {
	var (
		logger         *lagertest.TestLogger
		systemReporter *systemreporter.LogBased
		cmdRunner      *fake_command_runner.FakeCommandRunner
		threshold      time.Duration
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("reporter")
		cmdRunner = fake_command_runner.New()
		threshold = time.Second
	})

	JustBeforeEach(func() {
		systemReporter = systemreporter.NewLogBased(threshold, cmdRunner)
	})

	Describe("Report", func() {
		It("logs", func() {
			systemReporter.Report(logger, time.Minute)
			Expect(logger.Logs()).ToNot(BeEmpty())
		})

		It("reports the top running processes by cpu", func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "ps",
				Args: []string{"-aux", "--sort", "-pcpu"},
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write([]byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11"))
				Expect(err).NotTo(HaveOccurred())
				return nil
			})

			systemReporter.Report(logger, time.Minute)

			contents, err := ioutil.ReadAll(logger.Buffer())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(MatchRegexp(`"top_processes_by_cpu":"1\\n2\\n3\\n4\\n5\\n6\\n7\\n8\\n9\\n10"`))
		})

		It("reports the top running processes by memory", func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "ps",
				Args: []string{"-aux", "--sort", "-rss"},
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write([]byte("1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11"))
				Expect(err).NotTo(HaveOccurred())
				return nil
			})

			systemReporter.Report(logger, time.Minute)

			contents, err := ioutil.ReadAll(logger.Buffer())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(MatchRegexp(`"top_processes_by_memory":"1\\n2\\n3\\n4\\n5\\n6\\n7\\n8\\n9\\n10"`))
		})

		It("reports the dmesg", func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "dmesg",
				Args: []string{"-T"},
			}, func(cmd *exec.Cmd) error {
				for i := 1; i < 300; i++ {
					_, err := cmd.Stdout.Write([]byte(fmt.Sprintf("%d\n", i)))
					Expect(err).NotTo(HaveOccurred())
				}
				_, err := cmd.Stdout.Write([]byte("I've ran"))
				Expect(err).NotTo(HaveOccurred())
				return nil
			})

			systemReporter.Report(logger, time.Minute)

			report := logger.Logs()[0].Data["report"].(map[string]interface{})
			messages := strings.Split(report["dmesg"].(string), "\n")

			Expect(messages[len(messages)-1]).To(ContainSubstring("I've ran"))
			Expect(messages).To(HaveLen(250))
		})

		It("reports the iostat", func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "iostat",
				Args: []string{"-xzp"},
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write([]byte("I've ran"))
				Expect(err).NotTo(HaveOccurred())
				return nil
			})

			systemReporter.Report(logger, time.Minute)

			contents, err := ioutil.ReadAll(logger.Buffer())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("io_stat\":\"I've ran\""))
		})

		It("reports the mpstat", func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "mpstat",
				Args: []string{"-P", "ALL"},
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write([]byte("I've ran"))
				Expect(err).NotTo(HaveOccurred())
				return nil
			})

			systemReporter.Report(logger, time.Minute)

			contents, err := ioutil.ReadAll(logger.Buffer())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("mp_stat\":\"I've ran\""))
		})

		It("reports the vmstat", func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "vmstat",
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write([]byte("I've ran"))
				Expect(err).NotTo(HaveOccurred())
				return nil
			})

			systemReporter.Report(logger, time.Minute)

			contents, err := ioutil.ReadAll(logger.Buffer())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("vm_stat\":\"I've ran\""))
		})

		It("reports the pidstat", func() {
			cmdRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "pidstat",
			}, func(cmd *exec.Cmd) error {
				_, err := cmd.Stdout.Write([]byte("I've ran"))
				Expect(err).NotTo(HaveOccurred())
				return nil
			})

			systemReporter.Report(logger, time.Minute)

			contents, err := ioutil.ReadAll(logger.Buffer())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(ContainSubstring("pid_stat\":\"I've ran\""))
		})

		Context("when the threshold is < 0", func() {
			BeforeEach(func() {
				threshold = 0
			})

			It("doesn't report", func() {
				systemReporter.Report(logger, time.Millisecond)
				Expect(logger.Logs()).To(BeEmpty())
			})
		})

		Context("when the threshold is not reached", func() {
			It("doesn't report", func() {
				systemReporter.Report(logger, time.Millisecond)
				Expect(logger.Logs()).To(BeEmpty())
			})
		})
	})
})

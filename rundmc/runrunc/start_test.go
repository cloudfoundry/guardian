package runrunc_test

import (
	"errors"
	"io/ioutil"
	"os/exec"
	"path"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Start", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		bundlePath    string
		logger        *lagertest.TestLogger

		runner *runrunc.Starter
	)

	BeforeEach(func() {
		commandRunner = fake_command_runner.New()
		logger = lagertest.NewTestLogger("test")

		var err error
		bundlePath, err = ioutil.TempDir("", "bundle")
		Expect(err).NotTo(HaveOccurred())

		runner = runrunc.NewStarter("dadoo", "runc", commandRunner)
	})

	It("starts the container with runC passing the detach flag", func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "dadoo",
		}, func(cmd *exec.Cmd) error {
			Expect(ioutil.WriteFile(cmd.Args[2], []byte(""), 0700)).To(Succeed())
			_, err := cmd.ExtraFiles[0].Write([]byte{0})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())

		Expect(commandRunner.StartedCommands()[0].Path).To(Equal("dadoo"))
		Expect(commandRunner.StartedCommands()[0].Args).To(ConsistOf(
			"dadoo", "-log", path.Join(bundlePath, "start.log"), "run", "runc", bundlePath, "some-id",
		))
	})

	It("returns as soon as runc's exit status is returned on fd3", func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "dadoo",
		}, func(cmd *exec.Cmd) error {
			Expect(ioutil.WriteFile(cmd.Args[2], []byte(""), 0700)).To(Succeed())
			_, err := cmd.ExtraFiles[0].Write([]byte{0})
			Expect(err).NotTo(HaveOccurred())
			cmd.ExtraFiles[0].Close()

			return nil
		})

		waited := make(chan struct{})
		commandRunner.WhenWaitingFor(fake_command_runner.CommandSpec{
			Path: "dadoo",
		}, func(cmd *exec.Cmd) error {
			close(waited)
			time.Sleep(10 * time.Second)

			return nil
		})

		done := make(chan struct{})
		go func() {
			Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())
			close(done)
		}()

		Eventually(waited).Should(BeClosed(), "should wait on the dadoo process to avoid zombies")
		Eventually(done).Should(BeClosed(), "should not block on dadoo")
	})

	It("returns an error if the log file from start can't be read", func() {
		commandRunner.WhenRunning(fake_command_runner.CommandSpec{
			Path: "dadoo",
		}, func(cmd *exec.Cmd) error {
			_, err := cmd.ExtraFiles[0].Write([]byte{0})
			Expect(err).NotTo(HaveOccurred())
			return nil
		})

		Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError(ContainSubstring("start: read log file")))
	})

	Describe("forwarding logs from runC", func() {
		var (
			runcExitStatus  []byte
			dadooExitStatus error
			logs            string
		)

		BeforeEach(func() {
			runcExitStatus = []byte{0}
			dadooExitStatus = nil
			logs = `time="2016-03-02T13:56:38Z" level=warning msg="signal: potato"
				time="2016-03-02T13:56:38Z" level=error msg="fork/exec POTATO: no such file or directory"
				time="2016-03-02T13:56:38Z" level=fatal msg="Container start failed: [10] System error: fork/exec POTATO: no such file or directory"`
		})

		JustBeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "dadoo",
			}, func(cmd *exec.Cmd) error {
				Expect(ioutil.WriteFile(cmd.Args[2], []byte(logs), 0700)).To(Succeed())
				_, err := cmd.ExtraFiles[0].Write(runcExitStatus)
				Expect(err).NotTo(HaveOccurred())

				cmd.ExtraFiles[0].Close()
				return dadooExitStatus
			})
		})

		It("sends all the logs to the logger", func() {
			Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(Succeed())

			runcLogs := make([]lager.LogFormat, 0)
			for _, log := range logger.Logs() {
				if log.Message == "test.start.runc" {
					runcLogs = append(runcLogs, log)
				}
			}

			Expect(runcLogs).To(HaveLen(3))
			Expect(runcLogs[0].Data).To(HaveKeyWithValue("message", "signal: potato"))
		})

		Context("when launching runc with dadoo fails", func() {
			BeforeEach(func() {
				dadooExitStatus = errors.New("baboom")
			})

			It("returns an error", func() {
				Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("dadoo: baboom"))
			})
		})

		Context("when reading from fd3 fails", func() {
			BeforeEach(func() {
				runcExitStatus = nil
			})

			It("returns an error", func() {
				Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("dadoo: read fd3: EOF"))
			})
		})

		Context("when `runC start` itself fails", func() {
			BeforeEach(func() {
				runcExitStatus = []byte{3}
			})

			It("return an error including parsed logs when runC fails to start the container", func() {
				Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc start: exit status 3: Container start failed: [10] System error: fork/exec POTATO: no such file or directory"))
			})

			Context("when the log messages can't be parsed", func() {
				BeforeEach(func() {
					logs = `foo="'
					`
				})

				It("returns an error with only the exit status if the log can't be parsed", func() {
					Expect(runner.Start(logger, bundlePath, "some-id", garden.ProcessIO{})).To(MatchError("runc start: exit status 3: "))
				})
			})
		})
	})
})

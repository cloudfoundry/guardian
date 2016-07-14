package runrunc_test

import (
	"errors"
	"os/exec"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
	"github.com/cloudfoundry/gunk/command_runner/fake_command_runner"
	. "github.com/cloudfoundry/gunk/command_runner/fake_command_runner/matchers"
)

var _ = Describe("Stats", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runner        *fakes.FakeRuncCmdRunner
		runcBinary    *fakes.FakeRuncBinary
		logger        *lagertest.TestLogger

		statser *runrunc.Statser
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		commandRunner = fake_command_runner.New()
		runner = new(fakes.FakeRuncCmdRunner)
		logger = lagertest.NewTestLogger("test")

		statser = runrunc.NewStatser(runner, runcBinary)

		runcBinary.StatsCommandStub = func(id string, logFile string) *exec.Cmd {
			return exec.Command("funC-stats", "--log", logFile, id)
		}
		runner.RunAndLogStub = func(_ lager.Logger, fn runrunc.LoggingCmd) error {
			return commandRunner.Run(fn("potato.log"))
		}
	})

	Context("when runC reports valid JSON", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{
					"type": "stats",
					"data": {
						"cpu": {
							"usage": {
								"total": 1,
								"kernel": 2,
								"user": 3
							}
						},
						"memory": {
							"raw": {
								"active_anon": 1,
								"active_file": 2,
								"cache": 3,
								"hierarchical_memory_limit": 4,
								"inactive_anon": 5,
								"inactive_file": 6,
								"mapped_file": 7,
								"pgfault": 8,
								"pgmajfault": 9,
								"pgpgin": 10,
								"pgpgout": 11,
								"rss": 12,
								"rss_huge": 13,
								"total_active_anon": 14,
								"total_active_file": 15,
								"total_cache": 16,
								"total_inactive_anon": 17,
								"total_inactive_file": 18,
								"total_mapped_file": 19,
								"total_pgfault": 20,
								"total_pgmajfault": 21,
								"total_pgpgin": 22,
								"total_pgpgout": 23,
								"total_rss": 24,
								"total_rss_huge": 25,
								"total_unevictable": 26,
								"total_writeback": 27,
								"unevictable": 28,
								"writeback": 29,
								"swap": 30,
								"hierarchical_memsw_limit": 31,
								"total_swap": 32
							}
						}
					}
				}`))

				return nil
			})
		})

		It("parses the CPU stats", func() {
			stats, err := statser.Stats(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())

			Expect(stats.CPU).To(Equal(garden.ContainerCPUStat{
				Usage:  1,
				System: 2,
				User:   3,
			}))
		})

		It("parses the memory stats", func() {
			stats, err := statser.Stats(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())

			Expect(stats.Memory).To(Equal(garden.ContainerMemoryStat{
				ActiveAnon: 1,
				ActiveFile: 2,
				Cache:      3,
				HierarchicalMemoryLimit: 4,
				InactiveAnon:            5,
				InactiveFile:            6,
				MappedFile:              7,
				Pgfault:                 8,
				Pgmajfault:              9,
				Pgpgin:                  10,
				Pgpgout:                 11,
				Rss:                     12,
				TotalActiveAnon:         14,
				TotalActiveFile:         15,
				TotalCache:              16,
				TotalInactiveAnon:       17,
				TotalInactiveFile:       18,
				TotalMappedFile:         19,
				TotalPgfault:            20,
				TotalPgmajfault:         21,
				TotalPgpgin:             22,
				TotalPgpgout:            23,
				TotalRss:                24,
				TotalUnevictable:        26,
				Unevictable:             28,
				Swap:                    30,
				HierarchicalMemswLimit: 31,
				TotalSwap:              32,
				TotalUsageTowardLimit:  22,
			}))
		})

		It("forwards logs from runc", func() {
			_, err := statser.Stats(logger, "some-container")
			Expect(err).NotTo(HaveOccurred())

			Expect(commandRunner).To(HaveExecutedSerially(fake_command_runner.CommandSpec{
				Path: "funC-stats",
				Args: []string{"--log", "potato.log", "some-container"},
			}))
		})

	})

	Context("when runC reports invalid JSON", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{ banana potato banana potato }`))

				return nil
			})
		})

		It("should return an error", func() {
			_, err := statser.Stats(logger, "some-container")
			Expect(err).To(MatchError(ContainSubstring("decode stats")))
		})
	})

	Context("when runC fails", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				return errors.New("banana")
			})
		})

		It("returns an error", func() {
			_, err := statser.Stats(logger, "some-container")
			Expect(err).To(MatchError(ContainSubstring("runC stats: banana")))
		})
	})
})

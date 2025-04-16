package runrunc_test

import (
	"errors"
	"os/exec"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/commandrunner/fake_command_runner"
	. "code.cloudfoundry.org/commandrunner/fake_command_runner/matchers"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	fakes "code.cloudfoundry.org/guardian/rundmc/runrunc/runruncfakes"
)

var _ = Describe("Stats", func() {
	var (
		commandRunner *fake_command_runner.FakeCommandRunner
		runner        *fakes.FakeRuncCmdRunner
		runcBinary    *fakes.FakeRuncBinary
		theDepot      *fakes.FakeDepot
		processDepot  *fakes.FakeProcessDepot
		logger        *lagertest.TestLogger

		statser *runrunc.Statser

		stats  gardener.StatsContainerMetrics
		err    error
		handle = "some-handle"
	)

	BeforeEach(func() {
		runcBinary = new(fakes.FakeRuncBinary)
		theDepot = new(fakes.FakeDepot)
		processDepot = new(fakes.FakeProcessDepot)
		commandRunner = fake_command_runner.New()
		runner = new(fakes.FakeRuncCmdRunner)
		logger = lagertest.NewTestLogger("test")

		statser = runrunc.NewStatser(runner, runcBinary, theDepot, processDepot)

		runcBinary.StatsCommandStub = func(id string, logFile string) *exec.Cmd {
			return exec.Command("funC-stats", "--log", logFile, id)
		}

		runcBinary.StateCommandStub = func(id string, logFile string) *exec.Cmd {
			return exec.Command("funC-state", "--log", logFile, id)
		}

		runner.RunAndLogStub = func(_ lager.Logger, fn runrunc.LoggingCmd) error {
			return commandRunner.Run(fn("potato.log"))
		}
	})

	JustBeforeEach(func() {
		stats, err = statser.Stats(logger, handle)
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
								"total_inactive_file": 48,
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
								"total_swap": 32,
								"file": 8,
								"anon": 2,
								"swapcached": 20
							}
						},
						"pids": {
							"current": 33,
							"limit": 34
            }
					}
				}`))
				return nil
			})

			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-state",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{
					"created": "2018-08-17T11:05:57.464894007Z"
				}`))
				return nil
			})
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("parses the CPU stats", func() {
			Expect(stats.CPU).To(Equal(garden.ContainerCPUStat{
				Usage:  1,
				System: 2,
				User:   3,
			}))
		})

		It("shows container's age", func() {
			time.Sleep(5 * time.Millisecond)

			subsequentStats, err := statser.Stats(logger, handle)
			Expect(err).NotTo(HaveOccurred())

			Expect(subsequentStats.Age).To(BeNumerically(">", stats.Age))
		})

		It("parses the memory stats", func() {
			Expect(stats.Memory).To(Equal(garden.ContainerMemoryStat{
				ActiveAnon:              1,
				ActiveFile:              2,
				Cache:                   3,
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
				TotalInactiveFile:       48,
				TotalMappedFile:         19,
				TotalPgfault:            20,
				TotalPgmajfault:         21,
				TotalPgpgin:             22,
				TotalPgpgout:            23,
				TotalRss:                24,
				TotalUnevictable:        26,
				Unevictable:             28,
				Swap:                    30,
				HierarchicalMemswLimit:  31,
				TotalSwap:               32,
				TotalUsageTowardLimit:   24,
				File:                    8,
				Anon:                    2,
				SwapCached:              20,
			}))
		})

		It("parses the Pid stats", func() {
			Expect(stats.Pid).To(Equal(garden.ContainerPidStat{
				Current: 33,
				Max:     34,
			}))
		})

		It("forwards logs from runc", func() {
			Expect(commandRunner).To(HaveExecutedSerially(
				fake_command_runner.CommandSpec{
					Path: "funC-stats",
					Args: []string{"--log", "potato.log", handle},
				},
				fake_command_runner.CommandSpec{
					Path: "funC-state",
					Args: []string{"--log", "potato.log", handle},
				},
			))
		})
	})

	Context("when the JSON does not contain a 'created' field", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{ "type": "stats" }`))
				return nil
			})

			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-state",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{}`))
				return nil
			})

			theDepot.CreatedTimeReturns(time.Now().Add(-time.Hour), nil)
		})

		It("fetches the container age from the depot", func() {
			Expect(theDepot.CreatedTimeCallCount()).To(Equal(1))

			Expect(stats.Age).To(BeNumerically(">=", time.Hour))
			Expect(stats.Age).To(BeNumerically("<", time.Second*3603))
		})

		Context("when the container can't be found in the depot", func() {
			BeforeEach(func() {
				theDepot.CreatedTimeReturns(time.Time{}, depot.ErrDoesNotExist)
				processDepot.CreatedTimeReturns(time.Now().Add(-time.Hour), nil)
			})

			It("tries with the process depot", func() {
				Expect(processDepot.CreatedTimeCallCount()).To(Equal(1))

				Expect(stats.Age).To(BeNumerically(">=", time.Hour))
				Expect(stats.Age).To(BeNumerically("<", time.Second*3603))
			})

			Context("when the process depot fails too", func() {
				BeforeEach(func() {
					processDepot.CreatedTimeReturns(time.Time{}, errors.New("process-depot-error"))
				})

				It("forwards the error", func() {
					Expect(err).To(MatchError("process-depot-error"))
				})
			})
		})

		Context("when the depot fails", func() {
			BeforeEach(func() {
				theDepot.CreatedTimeReturns(time.Time{}, errors.New("Bad depot"))
			})

			It("forwards the error", func() {
				Expect(err).To(MatchError("Bad depot"))
			})
		})
	})

	Context("when runC stats reports invalid JSON", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{ banana potato banana potato }`))

				return nil
			})
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("decode stats")))
		})
	})

	Context("when runC state reports invalid JSON", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{}`))
				return nil
			})
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-state",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{ banana potato banana potato }`))

				return nil
			})
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring("decode state")))
		})
	})

	Context("when runC stats fails", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				return errors.New("banana")
			})
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("runC stats: banana")))
		})
	})

	Context("when runC state fails", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{}`))
				return nil
			})
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-state",
			}, func(cmd *exec.Cmd) error {
				return errors.New("banana")
			})
		})

		It("returns an error", func() {
			Expect(err).To(MatchError(ContainSubstring("runC state: banana")))
		})
	})

	Context("when runC stats reports inconsistent memory values", func() {
		BeforeEach(func() {
			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-stats",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{
					"type": "stats",
					"data": {
						"memory": {
							"raw": {
								"total_rss": 1,
								"total_cache": 2,
								"total_swap": 3,
								"total_inactive_file": 7
							}
						}
					}
				}`))

				return nil
			})

			commandRunner.WhenRunning(fake_command_runner.CommandSpec{
				Path: "funC-state",
			}, func(cmd *exec.Cmd) error {
				cmd.Stdout.Write([]byte(`{
					"created": "2018-08-17T11:05:57.464894007Z"
				}`))
				return nil
			})
		})

		It("doesn't underflow", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(stats.Memory.TotalUsageTowardLimit).To(BeZero())
		})
	})
})

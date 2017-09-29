package integration_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/config"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	"code.cloudfoundry.org/grootfs/integration/runner"
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	var (
		fakeMetronPort   uint16
		fakeMetron       *testhelpers.FakeMetron
		fakeMetronClosed chan struct{}
		spec             groot.CreateSpec
		randomImageID    string
	)

	BeforeEach(func() {
		fakeMetronPort = uint16(5000 + GinkgoParallelNode())

		fakeMetron = testhelpers.NewFakeMetron(fakeMetronPort)
		Expect(fakeMetron.Listen()).To(Succeed())

		fakeMetronClosed = make(chan struct{})
		go func() {
			defer GinkgoRecover()
			Expect(fakeMetron.Run()).To(Succeed())
			close(fakeMetronClosed)
		}()

		randomImageID = testhelpers.NewRandomID()
		spec = groot.CreateSpec{
			ID:        "my-id",
			BaseImage: "docker:///cfgarden/empty:v0.1.0",
			Mount:     mountByDefault(),
		}
	})

	AfterEach(func() {
		Expect(fakeMetron.Stop()).To(Succeed())
		Eventually(fakeMetronClosed).Should(BeClosed())
	})

	Describe("Create", func() {
		It("emits the total creation time", func() {
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageCreationTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageCreationTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits the unpack time", func() {
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("UnpackTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("UnpackTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits the locking time", func() {
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("SharedLockingTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("SharedLockingTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits store usage", func() {
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("StoreUsage")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("StoreUsage"))
			Expect(*metrics[0].Unit).To(Equal("bytes"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits the success count", func() {
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var counterEvents []events.CounterEvent
			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-create.run")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-create.run"))

			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-create.run.success")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-create.run.success"))
		})

		Describe("--with-clean", func() {
			BeforeEach(func() {
				spec.DiskLimit = 10000000000
				_, err := Runner.Create(spec)
				Expect(err).NotTo(HaveOccurred())

				writeMegabytes(filepath.Join(StorePath, store.TempDirName, "hello"), 100)
				writeMegabytes(filepath.Join(StorePath, store.MetaDirName, "hello"), 100)
			})

			It("emits the percentage of disk used by groot cache", func() {
				spec.ID = randomImageID
				_, err := Runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
					WithClean().Create(spec)
				Expect(err).NotTo(HaveOccurred())

				var metrics []events.ValueMetric
				Eventually(func() []events.ValueMetric {
					metrics = fakeMetron.ValueMetricsFor("DiskCachePercentage")
					return metrics
				}).Should(HaveLen(1))

				Expect(*metrics[0].Name).To(Equal("DiskCachePercentage"))
				Expect(*metrics[0].Unit).To(Equal("percentage"))
				Expect(*metrics[0].Value).To(BeNumerically("~", 20, 1))
			})

			It("emits the percentage of disk committed to the groot cache", func() {
				integration.SkipIfNotXFS(Driver)

				spec.ID = randomImageID
				_, err := Runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
					WithClean().Create(spec)
				Expect(err).NotTo(HaveOccurred())

				var metrics []events.ValueMetric
				Eventually(func() []events.ValueMetric {
					metrics = fakeMetron.ValueMetricsFor("DiskCommittedPercentage")
					return metrics
				}).Should(HaveLen(1))

				Expect(*metrics[0].Name).To(Equal("DiskCommittedPercentage"))
				Expect(*metrics[0].Unit).To(Equal("percentage"))
				Expect(*metrics[0].Value).To(BeNumerically("~", 940, 1))
			})
		})

		Context("when create fails", func() {
			BeforeEach(func() {
				spec.BaseImage = "not-here"
			})

			It("emits an error event", func() {
				_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
				Expect(err).To(HaveOccurred())

				var errors []events.Error
				Eventually(func() []events.Error {
					errors = fakeMetron.Errors()
					return errors
				}).Should(HaveLen(1))

				Expect(*errors[0].Source).To(Equal("grootfs-error.create"))
				Expect(*errors[0].Message).To(ContainSubstring("stat not-here: no such file or directory"))
			})

			It("emits the fail count", func() {
				_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
				Expect(err).To(HaveOccurred())

				var counterEvents []events.CounterEvent
				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-create.run")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-create.run"))

				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-create.run.fail")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-create.run.fail"))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			_, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("emits the total deletion time", func() {
			err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Delete("my-id")
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageDeletionTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageDeletionTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits the sucess count", func() {
			err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Delete("my-id")
			Expect(err).NotTo(HaveOccurred())

			var counterEvents []events.CounterEvent
			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-delete.run")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-delete.run"))

			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-delete.run.success")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-delete.run.success"))
		})

		Context("when delete fails", func() {
			var runner runner.Runner

			BeforeEach(func() {
				runner = Runner.RunningAsUser(GrootUID+1, GrootGID+1)
			})

			It("emits an error event", func() {
				err := runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
					Delete("my-id")
				Expect(err).To(HaveOccurred())

				var errors []events.Error
				Eventually(func() []events.Error {
					errors = fakeMetron.Errors()
					return errors
				}).Should(HaveLen(1))

				Expect(*errors[0].Source).To(Equal("grootfs-error.delete"))
				Expect(*errors[0].Message).To(ContainSubstring("permission denied"))
			})

			It("emits the fail count", func() {
				err := runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
					Delete("my-id")
				Expect(err).To(HaveOccurred())

				var counterEvents []events.CounterEvent
				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-delete.run")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-delete.run"))

				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-delete.run.fail")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-delete.run.fail"))
			})
		})
	})

	Describe("Stats", func() {
		BeforeEach(func() {
			_, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("emits the total time for metrics command", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Stats("my-id")
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageStatsTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageStatsTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits the sucess count", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Stats("my-id")
			Expect(err).NotTo(HaveOccurred())

			var counterEvents []events.CounterEvent
			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-stats.run")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-stats.run"))

			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-stats.run.success")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-stats.run.success"))
		})

		Context("when stats fails", func() {
			It("emits an error event", func() {
				_, err := Runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
					Stats("some-other-id")
				Expect(err).To(HaveOccurred())

				var errors []events.Error
				Eventually(func() []events.Error {
					errors = fakeMetron.Errors()
					return errors
				}).Should(HaveLen(1))

				Expect(*errors[0].Source).To(Equal("grootfs-error.stats"))
				Expect(*errors[0].Message).To(ContainSubstring("not found"))
			})

			It("emits the fail count", func() {
				_, err := Runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
					Stats("some-other-id")
				Expect(err).To(HaveOccurred())

				var counterEvents []events.CounterEvent
				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-stats.run")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-stats.run"))

				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-stats.run.fail")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-stats.run.fail"))
			})
		})
	})

	Describe("Clean", func() {
		BeforeEach(func() {
			spec.DiskLimit = 10000000000
			_, err := Runner.Create(spec)
			Expect(err).NotTo(HaveOccurred())

			writeMegabytes(filepath.Join(StorePath, store.TempDirName, "hello"), 100)
			writeMegabytes(filepath.Join(StorePath, store.MetaDirName, "hello"), 100)
		})

		It("emits the percentage of disk used by groot cache", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("DiskCachePercentage")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("DiskCachePercentage"))
			Expect(*metrics[0].Unit).To(Equal("percentage"))
			Expect(*metrics[0].Value).To(BeNumerically("~", 20, 1))
		})

		It("emits the percentage of disk committed to groot images", func() {
			integration.SkipIfNotXFS(Driver)

			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("DiskCommittedPercentage")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("DiskCommittedPercentage"))
			Expect(*metrics[0].Unit).To(Equal("percentage"))
			Expect(*metrics[0].Value).To(BeNumerically("~", 940, 1))
		})

		It("emits the space taken by layers that can be removed", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("DiskPurgeableCachePercentage")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("DiskPurgeableCachePercentage"))
			Expect(*metrics[0].Unit).To(Equal("percentage"))
			Expect(*metrics[0].Value).To(BeZero())
		})

		It("emits the total clean time", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ImageCleanTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ImageCleanTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits the locking time", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("ExclusiveLockingTime")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("ExclusiveLockingTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits the success count", func() {
			_, err := Runner.
				WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var counterEvents []events.CounterEvent
			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-clean.run")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-clean.run"))

			Eventually(func() []events.CounterEvent {
				counterEvents = fakeMetron.CounterEvents("grootfs-clean.run.success")
				return counterEvents
			}).Should(HaveLen(1))
			Expect(*counterEvents[0].Name).To(Equal("grootfs-clean.run.success"))
		})

		It("emits store usage", func() {
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("StoreUsage")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("StoreUsage"))
			Expect(*metrics[0].Unit).To(Equal("bytes"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		Context("when clean fails", func() {
			var runner runner.Runner

			BeforeEach(func() {
				runner = Runner.RunningAsUser(GrootUID+1, GrootGID+1)
			})

			It("emits an error event", func() {
				_, err := runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
				Expect(err).To(HaveOccurred())

				var errors []events.Error
				Eventually(func() []events.Error {
					errors = fakeMetron.Errors()
					return errors
				}).Should(HaveLen(1))

				Expect(*errors[0].Source).To(Equal("grootfs-error.clean"))
				Expect(*errors[0].Message).To(ContainSubstring("permission denied"))
			})

			It("emits the fail count", func() {
				_, err := runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Clean(0)
				Expect(err).To(HaveOccurred())

				var counterEvents []events.CounterEvent
				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-clean.run")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-clean.run"))

				Eventually(func() []events.CounterEvent {
					counterEvents = fakeMetron.CounterEvents("grootfs-clean.run.fail")
					return counterEvents
				}).Should(HaveLen(1))
				Expect(*counterEvents[0].Name).To(Equal("grootfs-clean.run.fail"))
			})
		})
	})

	Describe("--config global flag", func() {
		var (
			configDir string
		)

		BeforeEach(func() {
			var err error
			configDir, err = ioutil.TempDir("", "")
			Expect(err).NotTo(HaveOccurred())

			cfg := config.Config{
				MetronEndpoint: fmt.Sprintf("127.0.0.1:%d", fakeMetronPort),
			}

			Expect(Runner.SetConfig(cfg)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(configDir)).To(Succeed())
		})

		Describe("metron endpoint", func() {
			It("uses the metron agent from the config file", func() {
				_, err := Runner.Create(spec)
				Expect(err).NotTo(HaveOccurred())

				var metrics []events.ValueMetric
				Eventually(func() []events.ValueMetric {
					metrics = fakeMetron.ValueMetricsFor("ImageCreationTime")
					return metrics
				}).Should(HaveLen(1))

				Expect(*metrics[0].Name).To(Equal("ImageCreationTime"))
				Expect(*metrics[0].Unit).To(Equal("nanos"))
				Expect(*metrics[0].Value).NotTo(BeZero())
			})
		})
	})
})

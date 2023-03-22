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
	"code.cloudfoundry.org/grootfs/store"
	"code.cloudfoundry.org/grootfs/testhelpers"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	var (
		fakeMetronPort   uint16
		fakeMetron       *testhelpers.FakeMetron
		fakeMetronClosed chan struct{}
		spec             groot.CreateSpec
	)

	BeforeEach(func() {
		fakeMetronPort = uint16(5000 + GinkgoParallelProcess())

		fakeMetron = testhelpers.NewFakeMetron(fakeMetronPort)
		Expect(fakeMetron.Listen()).To(Succeed())

		fakeMetronClosed = make(chan struct{})
		go func() {
			defer GinkgoRecover()
			Expect(fakeMetron.Run()).To(Succeed())
			close(fakeMetronClosed)
		}()

		spec = groot.CreateSpec{
			ID:           "my-id",
			BaseImageURL: integration.String2URL("docker:///cfgarden/empty:v0.1.0"),
			Mount:        mountByDefault(),
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

		It("emits grootfs unused layers size", func() {
			spec.BaseImageURL = integration.String2URL("docker:///cfgarden/garden-busybox")
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("UnusedLayersSize")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("UnusedLayersSize"))
			Expect(*metrics[0].Unit).To(Equal("bytes"))
			Expect(*metrics[0].Value).To(BeZero())
		})

		It("emits grootfs disk space committed to quotas in MB", func() {
			spec.DiskLimit = 1 * 1024 * 1024
			spec.ExcludeBaseImageFromQuota = true

			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			spec.ID = "my-id-2"
			spec.DiskLimit = 2 * 1024 * 1024
			_, err = Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("CommittedQuotaInBytes")
				return metrics
			}).Should(HaveLen(2))

			Expect(*metrics[0].Name).To(Equal("CommittedQuotaInBytes"))
			Expect(*metrics[0].Unit).To(Equal("bytes"))
			Expect(*metrics[0].Value).To(BeEquivalentTo(1 * 1024 * 1024))

			Expect(*metrics[1].Name).To(Equal("CommittedQuotaInBytes"))
			Expect(*metrics[1].Unit).To(Equal("bytes"))
			Expect(*metrics[1].Value).To(BeEquivalentTo(3 * 1024 * 1024))
		})

		It("emits grootfs disk space used by volumes", func() {
			spec.BaseImageURL = integration.String2URL("docker:///cfgarden/garden-busybox")
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("DownloadedLayersSizeInBytes")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("DownloadedLayersSizeInBytes"))
			Expect(*metrics[0].Unit).To(Equal("bytes"))
			Expect(*metrics[0].Value).To(BeNumerically("~", 2*1024*1024, 1*1024*1024))
		})

		Describe("--with-clean", func() {
			BeforeEach(func() {
				spec.BaseImageURL = integration.String2URL("docker:///cfgarden/garden-busybox")
				spec.CleanOnCreate = true
			})

			It("emits grootfs unused layers size", func() {
				_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).Create(spec)
				Expect(err).NotTo(HaveOccurred())

				var metrics []events.ValueMetric
				Eventually(func() []events.ValueMetric {
					metrics = fakeMetron.ValueMetricsFor("UnusedLayersSize")
					return metrics
				}).Should(HaveLen(1))

				Expect(*metrics[0].Name).To(Equal("UnusedLayersSize"))
				Expect(*metrics[0].Unit).To(Equal("bytes"))
				Expect(*metrics[0].Value).To(BeZero())
			})
		})
	})

	Describe("Delete", func() {
		JustBeforeEach(func() {
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

		Context("with a non-empty base image", func() {
			BeforeEach(func() {
				spec.BaseImageURL = integration.String2URL("docker:///cfgarden/garden-busybox")
			})

			It("emits a positive unused layers size", func() {
				err := Runner.
					WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
					Delete("my-id")
				Expect(err).NotTo(HaveOccurred())

				var metrics []events.ValueMetric
				Eventually(func() []events.ValueMetric {
					metrics = fakeMetron.ValueMetricsFor("UnusedLayersSize")
					return metrics
				}).Should(HaveLen(1))

				Expect(*metrics[0].Name).To(Equal("UnusedLayersSize"))
				Expect(*metrics[0].Unit).To(Equal("bytes"))
				Expect(*metrics[0].Value).To(BeNumerically(">", 0))
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
			}).ShouldNot(BeEmpty())

			Expect(*metrics[0].Name).To(Equal("ExclusiveLockingTime"))
			Expect(*metrics[0].Unit).To(Equal("nanos"))
			Expect(*metrics[0].Value).NotTo(BeZero())
		})

		It("emits unused layers size", func() {
			_, err := Runner.WithMetronEndpoint(net.ParseIP("127.0.0.1"), fakeMetronPort).
				Clean(0)
			Expect(err).NotTo(HaveOccurred())

			var metrics []events.ValueMetric
			Eventually(func() []events.ValueMetric {
				metrics = fakeMetron.ValueMetricsFor("UnusedLayersSize")
				return metrics
			}).Should(HaveLen(1))

			Expect(*metrics[0].Name).To(Equal("UnusedLayersSize"))
			Expect(*metrics[0].Unit).To(Equal("bytes"))
			Expect(*metrics[0].Value).To(BeZero())
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

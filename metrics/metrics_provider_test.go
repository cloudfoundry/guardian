package metrics_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"code.cloudfoundry.org/guardian/metrics"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MetricsProvider", func() {
	var (
		logger    *lagertest.TestLogger
		depotPath string

		m *metrics.MetricsProvider
	)

	BeforeEach(func() {
		var err error

		depotPath, err = ioutil.TempDir("", "depotDirs")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(depotPath, "depot-1"), 0660)).To(Succeed())
		Expect(os.Mkdir(filepath.Join(depotPath, "depot-2"), 0660)).To(Succeed())
		Expect(os.Mkdir(filepath.Join(depotPath, "depot-3"), 0660)).To(Succeed())

		Expect(err).ToNot(HaveOccurred())
		logger = lagertest.NewTestLogger("test")
		m = metrics.NewMetricsProvider(logger, depotPath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depotPath)).To(Succeed())
	})

	It("should report the number of loop devices, backing store files and depotDirs", func() {
		Expect(m.NumCPU()).To(Equal(runtime.NumCPU()))
		Expect(m.NumGoroutine()).To(BeNumerically("~", runtime.NumGoroutine(), 2))
		Expect(m.LoopDevices()).To(Equal(0))
		Expect(m.BackingStores()).To(Equal(0))
		Expect(m.DepotDirs()).To(Equal(3))
	})

	Describe("UnkillableContainers", func() {
		Context("when flagging that a container was unkillable", func() {
			Describe("RegisterUnkillableContainer", func() {
				It("increments the unkillableContainerCount", func() {
					m.RegisterUnkillableContainer("my-container")
					Expect(m.UnkillableContainers()).To(Equal(1))
				})

				It("increments based off container ID, not call-count", func() {
					m.RegisterUnkillableContainer("my-container")
					m.RegisterUnkillableContainer("my-container")
					m.RegisterUnkillableContainer("my-container")
					Expect(m.UnkillableContainers()).To(Equal(1))
					m.RegisterUnkillableContainer("my-other-container")
					Expect(m.UnkillableContainers()).To(Equal(2))
				})

				Describe("RegisterKillableContainer", func() {
					It("decrements the unkillableContainerCount", func() {
						m.RegisterUnkillableContainer("my-container")
						Expect(m.UnkillableContainers()).To(Equal(1))
						m.RegisterKillableContainer("my-container")
						Expect(m.UnkillableContainers()).To(Equal(0))
					})

					It("is idempotent", func() {
						m.RegisterUnkillableContainer("my-container")
						Expect(m.UnkillableContainers()).To(Equal(1))
						m.RegisterKillableContainer("my-container")
						Expect(m.UnkillableContainers()).To(Equal(0))
						m.RegisterKillableContainer("my-container")
						Expect(m.UnkillableContainers()).To(Equal(0))
					})

					It("decrements based off container ID, not call-count", func() {
						m.RegisterUnkillableContainer("my-container")
						m.RegisterUnkillableContainer("my-other-container")
						Expect(m.UnkillableContainers()).To(Equal(2))

						m.RegisterKillableContainer("my-other-container")
						Expect(m.UnkillableContainers()).To(Equal(1))
					})

				})

				It("is thread safe", func() {
					wg := sync.WaitGroup{}
					for i := 0; i < 10; i++ {
						wg.Add(1)
						go func() {
							m.RegisterUnkillableContainer("my-container")
							m.RegisterKillableContainer("my-container")
							wg.Done()
						}()
					}
					wg.Wait()
					Expect(m.UnkillableContainers()).To(Equal(0))
				})
			})
		})
	})
})

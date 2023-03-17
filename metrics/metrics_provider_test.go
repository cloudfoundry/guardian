package metrics_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/guardian/metrics"
	"code.cloudfoundry.org/lager/v3/lagertest"

	. "github.com/onsi/ginkgo"
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
})

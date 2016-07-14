package metrics_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/guardian/metrics"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	var (
		logger           *lagertest.TestLogger
		backingStorePath string
		depotPath        string

		m metrics.Metrics
	)

	BeforeEach(func() {
		var err error

		backingStorePath, err = ioutil.TempDir("", "backing_stores")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(
			filepath.Join(backingStorePath, "bs-1"), []byte("test"), 0660,
		)).To(Succeed())
		Expect(ioutil.WriteFile(
			filepath.Join(backingStorePath, "bs-2"), []byte("test"), 0660,
		)).To(Succeed())

		depotPath, err = ioutil.TempDir("", "depotDirs")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Mkdir(filepath.Join(depotPath, "depot-1"), 0660)).To(Succeed())
		Expect(os.Mkdir(filepath.Join(depotPath, "depot-2"), 0660)).To(Succeed())
		Expect(os.Mkdir(filepath.Join(depotPath, "depot-3"), 0660)).To(Succeed())

		Expect(err).ToNot(HaveOccurred())
		logger = lagertest.NewTestLogger("test")
		m = metrics.NewMetrics(logger, backingStorePath, depotPath)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(depotPath)).To(Succeed())
		Expect(os.RemoveAll(backingStorePath)).To(Succeed())
	})

	It("should report the number of loop devices, backing store files and depotDirs", func() {
		Expect(m.NumCPU()).To(Equal(runtime.NumCPU()))
		Expect(m.NumGoroutine()).To(BeNumerically("~", runtime.NumGoroutine(), 2))
		Expect(m.LoopDevices()).NotTo(BeNil())
		Expect(m.BackingStores()).To(Equal(2))
		Expect(m.DepotDirs()).To(Equal(3))
	})

	Context("when the backing store path is empty", func() {
		It("reports BackingStores as -1 without doing any funny business", func() {
			m := metrics.NewMetrics(logger, "", depotPath)
			Expect(m.BackingStores()).To(Equal(-1))

			Expect(logger.LogMessages()).To(BeEmpty())
		})
	})
})

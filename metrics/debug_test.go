package metrics_test

import (
	"expvar"
	"net/http"
	"os"

	"code.cloudfoundry.org/guardian/metrics"
	fakes "code.cloudfoundry.org/guardian/metrics/metricsfakes"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Debug", func() {
	var (
		serverProc  ifrit.Process
		fakeMetrics *fakes.FakeMetrics
	)

	BeforeEach(func() {
		var err error

		fakeMetrics = new(fakes.FakeMetrics)
		fakeMetrics.NumCPUReturns(11)
		fakeMetrics.NumGoroutineReturns(888)
		fakeMetrics.LoopDevicesReturns(33)
		fakeMetrics.BackingStoresReturns(12)
		fakeMetrics.DepotDirsReturns(3)

		sink := lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG)
		serverProc, err = metrics.StartDebugServer("127.0.0.1:5123", sink, fakeMetrics)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		serverProc.Signal(os.Kill)
	})

	It("should report the number of loop devices, backing store files and depotDirs", func() {
		resp, err := http.Get("http://127.0.0.1:5123/debug/vars")
		Expect(err).ToNot(HaveOccurred())

		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		Expect(expvar.Get("loopDevices").String()).To(Equal("33"))
		Expect(expvar.Get("backingStores").String()).To(Equal("12"))
		Expect(expvar.Get("depotDirs").String()).To(Equal("3"))
		Expect(expvar.Get("numCPUS").String()).To(Equal("11"))
		Expect(expvar.Get("numGoRoutines").String()).To(Equal("888"))
	})
})

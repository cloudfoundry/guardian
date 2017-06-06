package metrics_test

import (
	"expvar"
	"net/http"
	"os"

	"code.cloudfoundry.org/guardian/metrics"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Debug", func() {
	var (
		serverProc ifrit.Process
	)

	BeforeEach(func() {
		var err error

		testMetrics := map[string]func() int{
			"metric1": func() int { return 33 },
			"metric2": func() int { return 12 },
		}

		sink := lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG)
		serverProc, err = metrics.StartDebugServer("127.0.0.1:5123", sink, testMetrics)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		serverProc.Signal(os.Kill)
	})

	It("should report the configured metrics", func() {
		resp, err := http.Get("http://127.0.0.1:5123/debug/vars")
		Expect(err).ToNot(HaveOccurred())

		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		Expect(expvar.Get("metric1").String()).To(Equal("33"))
		Expect(expvar.Get("metric2").String()).To(Equal("12"))
	})
})

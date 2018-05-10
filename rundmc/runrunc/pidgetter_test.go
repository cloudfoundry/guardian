package runrunc_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FilePidGetter", func() {
	var (
		pidGetter  runrunc.FilePidGetter
		depot      *rundmcfakes.FakeDepot
		logger     *lagertest.TestLogger
		bundlePath string

		pid       int
		pidGetErr error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		var err error
		bundlePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		depot = new(rundmcfakes.FakeDepot)
		depot.LookupReturns(bundlePath, nil)
		pidGetter = runrunc.FilePidGetter{Depot: depot}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		pid, pidGetErr = pidGetter.GetPid(logger, "container-handle")
	})

	It("looks up the container bundle path", func() {
		Expect(depot.LookupCallCount()).To(Equal(1))
		actualLogger, actualHandle := depot.LookupArgsForCall(0)
		Expect(actualLogger).To(Equal(logger))
		Expect(actualHandle).To(Equal("container-handle"))
	})

	Context("when looking up the container bundle path errors", func() {
		BeforeEach(func() {
			depot.LookupReturns("", errors.New("potato"))
		})

		It("propagates the error", func() {
			Expect(pidGetErr).To(MatchError("potato"))
		})
	})

	Context("when the pidfile exists", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "pidfile"), []byte("1234"), os.ModePerm)).To(Succeed())
		})

		It("reads the pid", func() {
			Expect(pid).To(Equal(1234))
		})
	})

	Context("when reading the pid file errors", func() {
		It("propagates the error", func() {
			Expect(os.IsNotExist(pidGetErr)).To(BeTrue())
		})
	})

	Context("when parsing the pid fails", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(bundlePath, "pidfile"), []byte("foo"), os.ModePerm)).To(Succeed())
		})

		It("propagates the error", func() {
			Expect(pidGetErr).To(MatchError(ContainSubstring("invalid syntax")))
		})
	})
})

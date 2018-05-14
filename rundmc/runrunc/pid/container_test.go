package pid_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/runrunc/pid"
	"code.cloudfoundry.org/guardian/rundmc/runrunc/pid/pidfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerPidGetter", func() {
	var (
		pidGetter     pid.ContainerPidGetter
		depot         *pidfakes.FakeDepot
		pidFileReader *pidfakes.FakePidFileReader

		logger     *lagertest.TestLogger
		bundlePath string

		returnedPid int
		pidGetErr   error
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		var err error
		bundlePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		depot = new(pidfakes.FakeDepot)
		depot.LookupReturns(bundlePath, nil)

		pidFileReader = new(pidfakes.FakePidFileReader)
		pidFileReader.PidReturns(1234, nil)

		pidGetter = pid.ContainerPidGetter{Depot: depot, PidFileReader: pidFileReader}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	JustBeforeEach(func() {
		returnedPid, pidGetErr = pidGetter.GetPid(logger, "container-handle")
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

	It("gets the pid, using the path returned from the depot with pidfile suffixed", func() {
		Expect(pidFileReader.PidCallCount()).To(Equal(1))
		actualPidFilePath := pidFileReader.PidArgsForCall(0)
		Expect(actualPidFilePath).To(Equal(filepath.Join(bundlePath, "pidfile")))

		Expect(returnedPid).To(Equal(1234))
	})

	Context("when reading the pid file returns an error", func() {
		BeforeEach(func() {
			pidFileReader.PidReturns(0, errors.New("error-reading-pid"))
		})

		It("returns the error", func() {
			Expect(pidGetErr).To(MatchError("error-reading-pid"))
		})
	})
})

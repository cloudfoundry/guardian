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

	Describe("GetPid", func() {
		JustBeforeEach(func() {
			returnedPid, pidGetErr = pidGetter.GetPid(logger, "container-handle")
		})

		It("looks up the container bundle path", func() {
			Expect(depot.LookupCallCount()).To(Equal(1))
			actualLogger, actualHandle := depot.LookupArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualHandle).To(Equal("container-handle"))
		})

		It("gets the pid using the path returned from the depot with '/pidfile' suffixed", func() {
			Expect(pidFileReader.PidCallCount()).To(Equal(1))
			actualPidFilePath := pidFileReader.PidArgsForCall(0)
			Expect(actualPidFilePath).To(Equal(filepath.Join(bundlePath, "pidfile")))

			Expect(returnedPid).To(Equal(1234))
		})

		When("looking up the container bundle path errors", func() {
			BeforeEach(func() {
				depot.LookupReturns("", errors.New("error-looking-up-bundle"))
			})

			It("propagates the error", func() {
				Expect(pidGetErr).To(MatchError("error-looking-up-bundle"))
			})
		})

		When("reading the pid file errors", func() {
			BeforeEach(func() {
				pidFileReader.PidReturns(0, errors.New("error-reading-pid"))
			})

			It("returns the error", func() {
				Expect(pidGetErr).To(MatchError("error-reading-pid"))
			})
		})
	})

	Describe("GetPeaPid", func() {
		JustBeforeEach(func() {
			returnedPid, pidGetErr = pidGetter.GetPeaPid(logger, "container-handle", "pea-id")
		})

		It("looks up the container bundle path", func() {
			Expect(depot.LookupCallCount()).To(Equal(1))
			actualLogger, actualHandle := depot.LookupArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualHandle).To(Equal("container-handle"))
		})

		It("gets the pid using the path returned from the depot with '/processes/pea-id/pidfile' suffixed", func() {
			Expect(pidFileReader.PidCallCount()).To(Equal(1))
			actualPidFilePath := pidFileReader.PidArgsForCall(0)
			Expect(actualPidFilePath).To(Equal(filepath.Join(bundlePath, "processes", "pea-id", "pidfile")))

			Expect(returnedPid).To(Equal(1234))
		})

		When("looking up the container bundle path errors", func() {
			BeforeEach(func() {
				depot.LookupReturns("", errors.New("error-looking-up-bundle"))
			})

			It("propagates the error", func() {
				Expect(pidGetErr).To(MatchError("error-looking-up-bundle"))
			})
		})

		When("reading the pid file errors", func() {
			BeforeEach(func() {
				pidFileReader.PidReturns(0, errors.New("error-reading-pid"))
			})

			It("returns the error", func() {
				Expect(pidGetErr).To(MatchError("error-reading-pid"))
			})
		})
	})
})

package execrunner_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/execrunner"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("ProcessDirDepot", func() {
	var (
		bundlePath      string
		log             *lagertest.TestLogger
		bundleLookupper *depotfakes.FakeBundleLookupper
		processDepot    execrunner.ProcessDirDepot
	)

	BeforeEach(func() {
		bundleLookupper = new(depotfakes.FakeBundleLookupper)

		var err error
		bundlePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		log = lagertest.NewTestLogger("test")
		processDepot = execrunner.NewProcessDirDepot(bundleLookupper)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(bundlePath)).To(Succeed())
	})

	Describe("CreateProcessDir", func() {
		var (
			processPath string
			err         error
		)

		BeforeEach(func() {
			bundleLookupper.LookupReturns(bundlePath, nil)
		})

		JustBeforeEach(func() {
			processPath, err = processDepot.CreateProcessDir(log, "sandbox-handle", "process-id")
		})

		It("creates the process directory in the depot", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(processPath).To(Equal(filepath.Join(bundlePath, "processes", "process-id")))
			Expect(processPath).To(BeADirectory())

			Expect(bundleLookupper.LookupCallCount()).To(Equal(1))
			_, actualSandboxHandle := bundleLookupper.LookupArgsForCall(0)
			Expect(actualSandboxHandle).To(Equal("sandbox-handle"))
		})

		When("looking up the bundle in the depot fails", func() {
			BeforeEach(func() {
				bundleLookupper.LookupReturns("", errors.New("lookup-error"))
			})

			It("fails", func() {
				Expect(err).To(MatchError("lookup-error"))
			})
		})
	})
})

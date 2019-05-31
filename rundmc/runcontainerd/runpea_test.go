package runcontainerd_test

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunContainerPea", func() {

	var (
		fakePeaManager      *runcontainerdfakes.FakePeaManager
		fakeProcessManager  *runcontainerdfakes.FakeProcessManager
		fakeBundleSaver     *depotfakes.FakeBundleSaver
		fakeBundleLookupper *runcontainerdfakes.FakeBundleLookupper

		bundlePath string

		runContainerPea *runcontainerd.RunContainerPea
	)

	BeforeEach(func() {
		fakePeaManager = new(runcontainerdfakes.FakePeaManager)
		fakeProcessManager = new(runcontainerdfakes.FakeProcessManager)
		fakeBundleSaver = new(depotfakes.FakeBundleSaver)
		fakeBundleLookupper = new(runcontainerdfakes.FakeBundleLookupper)

		var err error
		bundlePath, err = ioutil.TempDir("", "bundle")
		Expect(err).NotTo(HaveOccurred())
		fakeBundleLookupper.LookupReturns(bundlePath, nil)

		runContainerPea = &runcontainerd.RunContainerPea{
			PeaManager:      fakePeaManager,
			ProcessManager:  fakeProcessManager,
			BundleSaver:     fakeBundleSaver,
			BundleLookupper: fakeBundleLookupper,
		}
	})

	Describe("Run", func() {
		It("creates a container using process args", func() {
			runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), "test-process-id", goci.Bndl{}, "sandbox-id", garden.ProcessIO{}, false, strings.NewReader(""), nil)

			Expect(fakePeaManager.CreateCallCount()).To(Equal(1))
			_, actualProcessPath, actualProcessID, _ := fakePeaManager.CreateArgsForCall(0)
			Expect(actualProcessPath).To(Equal(filepath.Join(bundlePath, "processes", "test-process-id")))
			Expect(actualProcessID).To(Equal("test-process-id"))
		})

		When("the creator returns an error", func() {
			BeforeEach(func() {
				fakePeaManager.CreateReturns(errors.New("error-creating"))
			})

			It("returns the error", func() {
				_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), "test-process-id", goci.Bndl{}, "", garden.ProcessIO{}, false, strings.NewReader(""), nil)

				Expect(err).To(MatchError("error-creating"))
			})
		})
	})
})

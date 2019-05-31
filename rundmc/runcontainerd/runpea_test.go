package runcontainerd_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	"code.cloudfoundry.org/guardian/rundmc/execrunner/execrunnerfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("RunContainerPea", func() {

	var (
		fakePeaManager      *runcontainerdfakes.FakePeaManager
		fakeProcessManager  *runcontainerdfakes.FakeProcessManager
		fakeBundleSaver     *depotfakes.FakeBundleSaver
		fakeBundleLookupper *runcontainerdfakes.FakeBundleLookupper
		fakeProcessDepot    *execrunnerfakes.FakeProcessDepot

		bundlePath  string
		processID   string
		processPath string = "test-process-id"

		runContainerPea *runcontainerd.RunContainerPea
	)

	BeforeEach(func() {
		fakePeaManager = new(runcontainerdfakes.FakePeaManager)
		fakeProcessManager = new(runcontainerdfakes.FakeProcessManager)
		fakeBundleSaver = new(depotfakes.FakeBundleSaver)
		fakeBundleLookupper = new(runcontainerdfakes.FakeBundleLookupper)
		fakeProcessDepot = new(execrunnerfakes.FakeProcessDepot)

		var err error
		bundlePath, err = ioutil.TempDir("", "bundle")
		Expect(err).NotTo(HaveOccurred())
		fakeBundleLookupper.LookupReturns(bundlePath, nil)

		processPath = filepath.Join(bundlePath, "processes", processID)
		Expect(os.MkdirAll(processPath, 0700)).To(Succeed())
		fakeProcessDepot.CreateProcessDirReturns(processPath, nil)

		runContainerPea = &runcontainerd.RunContainerPea{
			PeaManager:      fakePeaManager,
			ProcessManager:  fakeProcessManager,
			BundleSaver:     fakeBundleSaver,
			BundleLookupper: fakeBundleLookupper,
			ProcessDepot:    fakeProcessDepot,
		}
	})

	Describe("RunPea", func() {
		It("creates a container using process args", func() {
			_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), processID, goci.Bndl{}, "sandbox-id", garden.ProcessIO{}, false, strings.NewReader(""), nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePeaManager.CreateCallCount()).To(Equal(1))
			_, actualProcessPath, actualProcessID, _ := fakePeaManager.CreateArgsForCall(0)
			Expect(actualProcessPath).To(Equal(processPath))
			Expect(actualProcessID).To(Equal(processID))
		})

		It("creates the process folder in the depot", func() {
			_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), processID, goci.Bndl{}, "sandbox-id", garden.ProcessIO{}, false, strings.NewReader(""), nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeProcessDepot.CreateProcessDirCallCount()).To(Equal(1))
			_, actualSandboxHandle, actualProcessID := fakeProcessDepot.CreateProcessDirArgsForCall(0)
			Expect(actualSandboxHandle).To(Equal("sandbox-id"))
			Expect(actualProcessID).To(Equal(processID))
		})

		It("saves the bundle in the depot", func() {
			_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), processID, goci.Bndl{Spec: specs.Spec{Version: "my-bundle"}}, "sandbox-id", garden.ProcessIO{}, false, strings.NewReader(""), nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBundleSaver.SaveCallCount()).To(Equal(1))
			actualProcessBundle, actualProcessPath := fakeBundleSaver.SaveArgsForCall(0)
			Expect(actualProcessBundle.Spec.Version).To(Equal("my-bundle"))
			Expect(actualProcessPath).To(Equal(processPath))
		})

		When("creating the process folder in the depot fails", func() {
			BeforeEach(func() {
				fakeProcessDepot.CreateProcessDirReturns("", errors.New("create-process-dir-error"))
			})

			It("fails", func() {
				_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), processID, goci.Bndl{Spec: specs.Spec{Version: "my-bundle"}}, "sandbox-id", garden.ProcessIO{}, false, strings.NewReader(""), nil)
				Expect(err).To(MatchError("create-process-dir-error"))
			})
		})

		When("saving the bundle in the depot fails", func() {
			BeforeEach(func() {
				fakeBundleSaver.SaveReturns(errors.New("save-error"))
			})

			It("fails", func() {
				_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), processID, goci.Bndl{Spec: specs.Spec{Version: "my-bundle"}}, "sandbox-id", garden.ProcessIO{}, false, strings.NewReader(""), nil)
				Expect(err).To(MatchError("save-error"))
			})
		})

		When("the creator returns an error", func() {
			BeforeEach(func() {
				fakePeaManager.CreateReturns(errors.New("error-creating"))
			})

			It("returns the error", func() {
				_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), processID, goci.Bndl{}, "", garden.ProcessIO{}, false, strings.NewReader(""), nil)

				Expect(err).To(MatchError("error-creating"))
			})
		})
	})
})

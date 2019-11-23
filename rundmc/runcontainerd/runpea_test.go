package runcontainerd_test

import (
	"errors"
	"strings"

	"code.cloudfoundry.org/garden"
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
		fakePeaManager     *runcontainerdfakes.FakePeaManager
		fakeProcessManager *runcontainerdfakes.FakeProcessManager
		fakeVolumizer      *runcontainerdfakes.FakeVolumizer

		processID string

		runContainerPea *runcontainerd.RunContainerPea
	)

	BeforeEach(func() {
		fakePeaManager = new(runcontainerdfakes.FakePeaManager)
		fakeProcessManager = new(runcontainerdfakes.FakeProcessManager)
		fakeVolumizer = new(runcontainerdfakes.FakeVolumizer)

		var err error
		Expect(err).NotTo(HaveOccurred())

		runContainerPea = runcontainerd.NewRunContainerPea(fakePeaManager, fakeProcessManager, fakeVolumizer, false)
	})

	Describe("RunPea", func() {
		It("creates a container using process args", func() {
			bundle := goci.Bndl{Spec: specs.Spec{Version: "test-version"}}
			_, err := runContainerPea.RunPea(lagertest.NewTestLogger("test-logger"), processID, bundle, "sandbox-id", garden.ProcessIO{}, false, strings.NewReader(""), nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakePeaManager.CreateCallCount()).To(Equal(1))
			_, _, actualProcessID, actualBundle, _ := fakePeaManager.CreateArgsForCall(0)
			Expect(actualProcessID).To(Equal(processID))
			Expect(actualBundle.Spec.Version).To(Equal("test-version"))
			Expect(actualBundle.Spec.Annotations["container-type"]).To(Equal("pea"))
			Expect(actualBundle.Spec.Annotations["sandbox-container"]).To(Equal("sandbox-id"))
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

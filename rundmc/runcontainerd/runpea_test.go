package runcontainerd_test

import (
	"errors"
	"strings"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RunContainerPea", func() {

	var (
		fakeCreator        *runcontainerdfakes.FakeCreator
		fakeProcessManager *runcontainerdfakes.FakeProcessManager

		runContainerPea *runcontainerd.RunContainerPea
	)

	BeforeEach(func() {
		fakeCreator = new(runcontainerdfakes.FakeCreator)
		fakeProcessManager = new(runcontainerdfakes.FakeProcessManager)

		runContainerPea = &runcontainerd.RunContainerPea{
			Creator:        fakeCreator,
			ProcessManager: fakeProcessManager,
		}
	})

	Describe("Run", func() {
		It("creates a container using process args", func() {
			runContainerPea.Run(lagertest.NewTestLogger("test-logger"), "test-process-id", "/process/path", "", "", garden.ProcessIO{}, false, strings.NewReader(""), nil)

			Expect(fakeCreator.CreateCallCount()).To(Equal(1))
			_, actualProcessPath, actualProcessID, _ := fakeCreator.CreateArgsForCall(0)
			Expect(actualProcessPath).To(Equal("/process/path"))
			Expect(actualProcessID).To(Equal("test-process-id"))
		})

		When("the creator returns an error", func() {
			BeforeEach(func() {
				fakeCreator.CreateReturns(errors.New("error-creating"))
			})

			It("returns the error", func() {
				_, err := runContainerPea.Run(lagertest.NewTestLogger("test-logger"), "test-process-id", "/process/path", "", "", garden.ProcessIO{}, false, strings.NewReader(""), nil)

				Expect(err).To(MatchError("error-creating"))
			})
		})
	})
})

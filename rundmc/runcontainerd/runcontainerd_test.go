package runcontainerd_test

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Runcontainerd", func() {
	var (
		nerdulator    *runcontainerdfakes.FakeNerdulator
		bundleLoader  *runcontainerdfakes.FakeBundleLoader
		runContainerd *runcontainerd.RunContainerd
	)

	BeforeEach(func() {
		nerdulator = new(runcontainerdfakes.FakeNerdulator)
		bundleLoader = new(runcontainerdfakes.FakeBundleLoader)
		runContainerd = runcontainerd.New(nerdulator, bundleLoader)
	})

	Describe("Create", func() {

		var (
			createErr        error
			id               string
			bundle           goci.Bndl
			createdContainer *runcontainerdfakes.FakeContainer
			createdTask      *runcontainerdfakes.FakeTask
		)

		BeforeEach(func() {
			id = "randomID"
			bundle = goci.Bndl{
				Spec: specs.Spec{
					Hostname: "test-hostname",
				},
			}
			bundleLoader.LoadReturns(bundle, nil)
			createdContainer = new(runcontainerdfakes.FakeContainer)
			createdTask = new(runcontainerdfakes.FakeTask)
			nerdulator.CreateContainerReturns(createdContainer, nil)
			nerdulator.CreateTaskReturns(createdTask, nil)
		})

		JustBeforeEach(func() {
			createErr = runContainerd.Create(nil, "", id, garden.ProcessIO{})
		})

		It("creates a containerd container, with the right args", func() {
			Expect(nerdulator.CreateContainerCallCount()).To(Equal(1))
			actualID, actualSpec := nerdulator.CreateContainerArgsForCall(0)
			Expect(actualID).To(Equal(id))
			Expect(actualSpec).To(Equal(bundle.Spec))
		})

		It("creates a containerd task, with the right args", func() {
			Expect(nerdulator.CreateTaskCallCount()).To(Equal(1))
			_, usedContainer := nerdulator.CreateTaskArgsForCall(0)
			Expect(usedContainer).To(Equal(createdContainer))
		})

		It("starts a containerd task, with the right args", func() {
			Expect(nerdulator.StartTaskCallCount()).To(Equal(1))
			usedTask := nerdulator.StartTaskArgsForCall(0)
			Expect(usedTask).To(Equal(createdTask))
		})

		Context("when loading the bundle returns an error", func() {
			BeforeEach(func() {
				bundleLoader.LoadReturns(goci.Bndl{}, errors.New("POTATO"))
			})

			It("returns an error", func() {
				Expect(createErr).To(MatchError("POTATO"))
			})
		})

		Context("when creating a containerd task errors", func() {
			BeforeEach(func() {
				nerdulator.CreateTaskReturns(nil, errors.New("BOOMS"))
			})

			It("bubbles up that error", func() {
				Expect(createErr).To(MatchError("BOOMS"))
			})
		})

		Context("when starting a containerd task errors", func() {
			BeforeEach(func() {
				nerdulator.StartTaskReturns(errors.New("HEY"))
			})

			It("bubbles up that error", func() {
				Expect(createErr).To(MatchError("HEY"))
			})
		})
	})

	Describe("State", func() {
		var (
			stateErr        error
			loadedContainer *runcontainerdfakes.FakeContainer
			loadedTask      *runcontainerdfakes.FakeTask
		)

		BeforeEach(func() {
			loadedContainer = new(runcontainerdfakes.FakeContainer)
			loadedTask = new(runcontainerdfakes.FakeTask)
			nerdulator.LoadContainerReturns(loadedContainer, nil)
			nerdulator.GetTaskReturns(loadedTask, nil)
			nerdulator.GetTaskPidReturns(1)
		})

		JustBeforeEach(func() {
			_, stateErr = runContainerd.State(nil, "some-id")
		})

		It("should load the container, with the correct args", func() {
			Expect(stateErr).NotTo(HaveOccurred())
			Expect(nerdulator.LoadContainerCallCount()).To(Equal(1))
			actualID := nerdulator.LoadContainerArgsForCall(0)
			Expect(actualID).To(Equal("some-id"))
		})

		It("should load the container's associated task, with the correct args", func() {
			Expect(stateErr).NotTo(HaveOccurred())
			Expect(nerdulator.GetTaskCallCount()).To(Equal(1))
			container := nerdulator.GetTaskArgsForCall(0)
			Expect(container).To(Equal(loadedContainer))
		})

		It("should load the task's pid, with the correct args", func() {
			Expect(stateErr).NotTo(HaveOccurred())
			task := nerdulator.GetTaskPidArgsForCall(0)
			Expect(task).To(Equal(loadedTask))
			Expect(nerdulator.GetTaskPidCallCount()).To(Equal(1))
		})

		Context("when loading the container fails", func() {
			BeforeEach(func() {
				nerdulator.LoadContainerReturns(nil, errors.New("BOOM"))
			})

			It("bubbles up that error", func() {
				Expect(stateErr).To(MatchError("BOOM"))
			})
		})

		Context("when loading the container's task fails", func() {
			BeforeEach(func() {
				nerdulator.GetTaskReturns(nil, errors.New("BOOM"))
			})

			It("bubbles up that error", func() {
				Expect(stateErr).To(MatchError("BOOM"))
			})
		})
	})
})

package runcontainerd_test

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
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

	Describe("Delete", func() {
		var deleteErr error

		JustBeforeEach(func() {
			deleteErr = runContainerd.Delete(nil, false, "container-id")
		})

		It("deletes the containerd container with the passed id", func() {
			Expect(nerdulator.DeleteCallCount()).To(Equal(1))
			_, actualID := nerdulator.DeleteArgsForCall(0)
			Expect(actualID).To(Equal("container-id"))
		})

		Context("when deleting a containerd container errors", func() {
			BeforeEach(func() {
				nerdulator.DeleteReturns(errors.New("could not delete"))
			})

			It("bubbles up that error", func() {
				Expect(deleteErr).To(MatchError("could not delete"))
			})
		})
	})

	Describe("State", func() {
		var (
			state    runrunc.State
			stateErr error
		)

		BeforeEach(func() {
			nerdulator.StateReturns(1, "running", nil)
		})

		JustBeforeEach(func() {
			state, stateErr = runContainerd.State(nil, "some-id")
		})

		It("fetches the container's state, with the correct args", func() {
			Expect(stateErr).NotTo(HaveOccurred())
			Expect(nerdulator.StateCallCount()).To(Equal(1))
			_, actualID := nerdulator.StateArgsForCall(0)
			Expect(actualID).To(Equal("some-id"))

			Expect(state.Pid).To(Equal(1))
			Expect(state.Status).To(Equal(runrunc.RunningStatus))
		})

		Context("when getting the state fails", func() {
			BeforeEach(func() {
				nerdulator.StateReturns(0, "", errors.New("BOOM"))
			})

			It("bubbles up that error", func() {
				Expect(stateErr).To(MatchError("BOOM"))
			})
		})
	})
})

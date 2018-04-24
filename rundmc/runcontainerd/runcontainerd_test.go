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
			id         string
			bundlePath string
			bundle     goci.Bndl

			createErr error
		)

		BeforeEach(func() {
			id = "container-id"
			bundlePath = "bundle-path"
			bundle = goci.Bndl{
				Spec: specs.Spec{
					Hostname: "test-hostname",
				},
			}
			bundleLoader.LoadStub = func(path string) (goci.Bndl, error) {
				if path == bundlePath {
					return bundle, nil
				}
				return goci.Bndl{}, nil
			}
		})

		JustBeforeEach(func() {
			createErr = runContainerd.Create(nil, bundlePath, id, garden.ProcessIO{})
		})

		It("creates the container with the passed containerID", func() {
			Expect(nerdulator.CreateCallCount()).To(Equal(1))
			_, actualID, _ := nerdulator.CreateArgsForCall(0)
			Expect(actualID).To(Equal(id))
		})

		It("creates the container using the spec from the loaded bundle", func() {
			Expect(nerdulator.CreateCallCount()).To(Equal(1))
			_, _, actualSpec := nerdulator.CreateArgsForCall(0)
			Expect(actualSpec).To(Equal(&bundle.Spec))
		})

		Context("when loading the bundle returns an error", func() {
			BeforeEach(func() {
				bundleLoader.LoadReturns(goci.Bndl{}, errors.New("EXPLODE"))
			})

			It("bubbles up that", func() {
				Expect(createErr).To(MatchError("EXPLODE"))
			})
		})

		Context("when creating the container returns an error", func() {
			BeforeEach(func() {
				nerdulator.CreateReturns(errors.New("EXPLODE"))
			})

			It("bubbles up that", func() {
				Expect(createErr).To(MatchError("EXPLODE"))
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
			state runrunc.State

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

package runcontainerd_test

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Runcontainerd", func() {
	var (
		logger        lager.Logger
		nerd          *runcontainerdfakes.FakeNerdContainerizer
		bundleLoader  *runcontainerdfakes.FakeBundleLoader
		runContainerd *runcontainerd.RunContainerd
		execer        *runcontainerdfakes.FakeExecer
		statser       *runcontainerdfakes.FakeStatser
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("potato")
		nerd = new(runcontainerdfakes.FakeNerdContainerizer)
		bundleLoader = new(runcontainerdfakes.FakeBundleLoader)
		execer = new(runcontainerdfakes.FakeExecer)
		statser = new(runcontainerdfakes.FakeStatser)
		runContainerd = runcontainerd.New(nerd, bundleLoader, execer, statser)
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
			Expect(nerd.CreateCallCount()).To(Equal(1))
			_, actualID, _ := nerd.CreateArgsForCall(0)
			Expect(actualID).To(Equal(id))
		})

		It("creates the container using the spec from the loaded bundle", func() {
			Expect(nerd.CreateCallCount()).To(Equal(1))
			_, _, actualSpec := nerd.CreateArgsForCall(0)
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
				nerd.CreateReturns(errors.New("EXPLODE"))
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
			Expect(nerd.DeleteCallCount()).To(Equal(1))
			_, actualID := nerd.DeleteArgsForCall(0)
			Expect(actualID).To(Equal("container-id"))
		})

		Context("when deleting a containerd container errors", func() {
			BeforeEach(func() {
				nerd.DeleteReturns(errors.New("could not delete"))
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
			nerd.StateReturns(1, "running", nil)
		})

		JustBeforeEach(func() {
			state, stateErr = runContainerd.State(nil, "some-id")
		})

		It("fetches the container's state, with the correct args", func() {
			Expect(stateErr).NotTo(HaveOccurred())
			Expect(nerd.StateCallCount()).To(Equal(1))
			_, actualID := nerd.StateArgsForCall(0)
			Expect(actualID).To(Equal("some-id"))

			Expect(state.Pid).To(Equal(1))
			Expect(state.Status).To(Equal(runrunc.RunningStatus))
		})

		Context("when getting the state fails", func() {
			BeforeEach(func() {
				nerd.StateReturns(0, "", errors.New("BOOM"))
			})

			It("bubbles up that error", func() {
				Expect(stateErr).To(MatchError("BOOM"))
			})
		})
	})

	Describe("Exec", func() {
		var (
			spec    garden.ProcessSpec
			io      garden.ProcessIO
			process *gardenfakes.FakeProcess

			execProcess garden.Process
			execError   error
		)

		BeforeEach(func() {
			spec = garden.ProcessSpec{ID: "process-id"}
			io = garden.ProcessIO{}
			process = new(gardenfakes.FakeProcess)
			execer.ExecReturns(process, nil)
		})

		JustBeforeEach(func() {
			execProcess, execError = runContainerd.Exec(logger, "bundle-path", "some-id", spec, io)
		})

		It("delegates to execer", func() {
			Expect(execError).NotTo(HaveOccurred())
			Expect(execProcess).To(BeIdenticalTo(process))

			Expect(execer.ExecCallCount()).To(Equal(1))
			actualLogger, actualBundlePath, actualID, actualSpec, actualIO := execer.ExecArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualBundlePath).To(Equal("bundle-path"))
			Expect(actualID).To(Equal("some-id"))
			Expect(actualSpec).To(Equal(spec))
			Expect(actualIO).To(Equal(io))
		})

		Context("when the execer fails", func() {
			BeforeEach(func() {
				execer.ExecReturns(nil, errors.New("execer-failed"))
			})

			It("returns the execer error", func() {
				Expect(execError).To(MatchError("execer-failed"))
			})
		})
	})

	Describe("Attach", func() {
		var (
			io      garden.ProcessIO
			process *gardenfakes.FakeProcess

			attachProcess garden.Process
			attachError   error
		)

		BeforeEach(func() {
			io = garden.ProcessIO{}
			process = new(gardenfakes.FakeProcess)
			execer.AttachReturns(process, nil)
		})

		JustBeforeEach(func() {
			attachProcess, attachError = runContainerd.Attach(logger, "bundle-path", "some-id", "some-proc-id", io)
		})

		It("delegates to execer", func() {
			Expect(attachError).NotTo(HaveOccurred())
			Expect(attachProcess).To(BeIdenticalTo(process))

			Expect(execer.AttachCallCount()).To(Equal(1))
			actualLogger, actualBundlePath, actualContainerID, actualProcessID, actualIO := execer.AttachArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualBundlePath).To(Equal("bundle-path"))
			Expect(actualContainerID).To(Equal("some-id"))
			Expect(actualProcessID).To(Equal("some-proc-id"))
			Expect(actualIO).To(Equal(io))
		})

		Context("when the execer fails", func() {
			BeforeEach(func() {
				execer.AttachReturns(nil, errors.New("execer-failed"))
			})

			It("returns the execer error", func() {
				Expect(attachError).To(MatchError("execer-failed"))
			})
		})
	})

	Describe("Stats", func() {
		var (
			metrics    gardener.ActualContainerMetrics
			metricsErr error
		)

		BeforeEach(func() {
			statser.StatsReturns(gardener.ActualContainerMetrics{CPU: garden.ContainerCPUStat{Usage: 123}}, nil)
		})

		JustBeforeEach(func() {
			metrics, metricsErr = runContainerd.Stats(logger, "some-id")
		})

		It("it succeeds", func() {
			Expect(metricsErr).NotTo(HaveOccurred())
		})

		It("delegates to statser with the correct arguments", func() {
			Expect(statser.StatsCallCount()).To(Equal(1))
			actualLogger, actualContainerId := statser.StatsArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualContainerId).To(Equal("some-id"))
		})

		It("returns a statser metric", func() {
			Expect(metrics.CPU.Usage).To(Equal(uint64(123)))
		})

		Context("statser fails", func() {
			BeforeEach(func() {
				statser.StatsReturns(gardener.ActualContainerMetrics{}, errors.New("statser-failure"))
			})

			It("returns the statser error", func() {
				Expect(metricsErr).To(MatchError("statser-failure"))
			})
		})
	})
})

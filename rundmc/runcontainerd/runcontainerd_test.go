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
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/rundmc/users/usersfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = FDescribe("Runcontainerd", func() {
	var (
		logger           lager.Logger
		containerManager *runcontainerdfakes.FakeContainerManager
		processManager   *runcontainerdfakes.FakeProcessManager
		bundleLoader     *runcontainerdfakes.FakeBundleLoader
		runContainerd    *runcontainerd.RunContainerd
		execer           *runcontainerdfakes.FakeExecer
		statser          *runcontainerdfakes.FakeStatser
		processBuilder   *runcontainerdfakes.FakeProcessBuilder
		userLookupper    *usersfakes.FakeUserLookupper
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-logger")
		containerManager = new(runcontainerdfakes.FakeContainerManager)
		processManager = new(runcontainerdfakes.FakeProcessManager)
		bundleLoader = new(runcontainerdfakes.FakeBundleLoader)
		execer = new(runcontainerdfakes.FakeExecer)
		statser = new(runcontainerdfakes.FakeStatser)
		processBuilder = new(runcontainerdfakes.FakeProcessBuilder)
		userLookupper = new(usersfakes.FakeUserLookupper)

		runContainerd = runcontainerd.New(containerManager, processManager, bundleLoader, processBuilder, userLookupper, execer, statser, false)
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
			Expect(containerManager.CreateCallCount()).To(Equal(1))
			_, actualID, _ := containerManager.CreateArgsForCall(0)
			Expect(actualID).To(Equal(id))
		})

		It("creates the container using the spec from the loaded bundle", func() {
			Expect(containerManager.CreateCallCount()).To(Equal(1))
			_, _, actualSpec := containerManager.CreateArgsForCall(0)
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
				containerManager.CreateReturns(errors.New("EXPLODE"))
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
			Expect(containerManager.DeleteCallCount()).To(Equal(1))
			_, actualID := containerManager.DeleteArgsForCall(0)
			Expect(actualID).To(Equal("container-id"))
		})

		Context("when deleting a containerd container errors", func() {
			BeforeEach(func() {
				containerManager.DeleteReturns(errors.New("could not delete"))
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
			containerManager.StateReturns(1, "running", nil)
		})

		JustBeforeEach(func() {
			state, stateErr = runContainerd.State(nil, "some-id")
		})

		It("fetches the container's state, with the correct args", func() {
			Expect(stateErr).NotTo(HaveOccurred())
			Expect(containerManager.StateCallCount()).To(Equal(1))
			_, actualID := containerManager.StateArgsForCall(0)
			Expect(actualID).To(Equal("some-id"))

			Expect(state.Pid).To(Equal(1))
			Expect(state.Status).To(Equal(runrunc.RunningStatus))
		})

		Context("when getting the state fails", func() {
			BeforeEach(func() {
				containerManager.StateReturns(0, "", errors.New("BOOM"))
			})

			It("bubbles up that error", func() {
				Expect(stateErr).To(MatchError("BOOM"))
			})
		})
	})

	Describe("Exec", func() {
		var (
			containerID string
			bundlePath  string
			processSpec garden.ProcessSpec
			execErr     error
			bundle      goci.Bndl
			fakeProcess *gardenfakes.FakeProcess
			execProcess garden.Process
			processIO   garden.ProcessIO
		)

		BeforeEach(func() {
			fakeProcess = new(gardenfakes.FakeProcess)
			execer.ExecReturns(fakeProcess, nil)
			containerID = "container-id"
			bundlePath = "bundle-path"
			processSpec = garden.ProcessSpec{
				ID:   "test-process-id",
				User: "alice",
			}
			processBuilder.BuildProcessReturns(&specs.Process{
				Args: []string{"test-binary"},
			})
			bundle = goci.Bndl{Spec: specs.Spec{Hostname: "test-hostname"}}
			bundleLoader.LoadStub = func(path string) (goci.Bndl, error) {
				if path == bundlePath {
					return bundle, nil
				}
				return goci.Bndl{}, nil
			}
			processIO = garden.ProcessIO{
				Stdout: gbytes.NewBuffer(),
			}
		})

		JustBeforeEach(func() {
			execProcess, execErr = runContainerd.Exec(logger, bundlePath, containerID, processSpec, processIO)
		})

		It("delegates to execer", func() {
			Expect(execer.ExecCallCount()).To(Equal(1))
			actualLogger, actualBundlePath, actualSandboxHandle, actualProcessSpec, actualIO := execer.ExecArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualBundlePath).To(Equal(bundlePath))
			Expect(actualSandboxHandle).To(Equal(containerID))
			Expect(actualProcessSpec).To(Equal(processSpec))
			Expect(actualIO).To(Equal(processIO))
			Expect(execProcess).To(Equal(fakeProcess))
		})

		Context("when the execer fails", func() {
			BeforeEach(func() {
				execer.ExecReturns(nil, errors.New("execer-failed"))
			})

			It("returns the execer error", func() {
				Expect(execErr).To(MatchError("execer-failed"))
			})
		})

		Context("when use_containerd_for_processes is enabled", func() {
			BeforeEach(func() {
				user := users.ExecUser{
					Uid:  1000,
					Gid:  1001,
					Home: "/home/alice",
				}
				userLookupper.LookupReturns(&user, nil)

				containerManager.GetContainerPIDReturns(1234, nil)
				containerManager.ExecReturns(nil)
				runContainerd = runcontainerd.New(containerManager, processManager, bundleLoader, processBuilder, userLookupper, execer, statser, true)
			})

			It("passes the logger through", func() {
				Expect(containerManager.ExecCallCount()).To(Equal(1))
				actualLogger, _, _, _, _ := containerManager.ExecArgsForCall(0)
				Expect(actualLogger).To(Equal(logger))
			})

			It("passes the io through", func() {
				Expect(containerManager.ExecCallCount()).To(Equal(1))
				_, _, _, _, actualIO := containerManager.ExecArgsForCall(0)
				Expect(actualIO).To(Equal(processIO))
			})

			It("creates the process on the passed container", func() {
				Expect(containerManager.ExecCallCount()).To(Equal(1))
				_, actualContainerID, _, _, _ := containerManager.ExecArgsForCall(0)
				Expect(actualContainerID).To(Equal(containerID))
			})

			It("creates the process with the provided processID", func() {
				Expect(containerManager.ExecCallCount()).To(Equal(1))
				_, _, actualProcessID, _, _ := containerManager.ExecArgsForCall(0)
				Expect(actualProcessID).To(Equal(processSpec.ID))
			})

			It("converts the garden process to an OCI process", func() {
				Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
				passedBundle, passedGardenProcessSpec, _, _ := processBuilder.BuildProcessArgsForCall(0)
				Expect(passedBundle).To(Equal(bundle))
				passedGardenProcessSpec.Dir = processSpec.Dir
				Expect(passedGardenProcessSpec).To(Equal(processSpec))
			})

			It("creates the process with the converted process spec", func() {
				Expect(containerManager.ExecCallCount()).To(Equal(1))
				_, _, _, actualProcessSpec, _ := containerManager.ExecArgsForCall(0)
				Expect(actualProcessSpec.Args).To(Equal([]string{"test-binary"}))
			})

			It("creates the process with the resolved user", func() {
				_, actualContainerId := containerManager.GetContainerPIDArgsForCall(0)
				Expect(actualContainerId).To(Equal("container-id"))

				Expect(userLookupper.LookupCallCount()).To(Equal(1))
				passedRootfs, passedUserId := userLookupper.LookupArgsForCall(0)
				Expect(passedUserId).To(Equal("alice"))
				Expect(passedRootfs).To(Equal("/proc/1234/root"))

				Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
				_, _, ociProcessUid, ociProcessGid := processBuilder.BuildProcessArgsForCall(0)
				Expect(ociProcessUid).To(Equal(1000))
				Expect(ociProcessGid).To(Equal(1001))
			})

			Context("when processSpec.ID is not set", func() {
				BeforeEach(func() {
					processSpec.ID = ""
				})

				It("generates an ID", func() {
					Expect(containerManager.ExecCallCount()).To(Equal(1))
					_, _, actualProcessID, _, _ := containerManager.ExecArgsForCall(0)
					Expect(actualProcessID).NotTo(Equal(""))
				})
			})

			Describe("the process itself", func() {
				BeforeEach(func() {
					processManager.WaitReturns(17, nil)
				})

				It("collects the exit code from the process manager", func() {
					exitStatus, err := execProcess.Wait()
					Expect(err).NotTo(HaveOccurred())
					Expect(exitStatus).To(Equal(17))

					Expect(processManager.WaitCallCount()).NotTo(BeZero())
					_, actualContainerID, actualProcessID := processManager.WaitArgsForCall(0)
					Expect(actualContainerID).To(Equal("container-id"))
					Expect(actualProcessID).To(Equal("test-process-id"))
				})

				Context("when waiting on the process fails", func() {
					BeforeEach(func() {
						processManager.WaitReturns(17, errors.New("Oh no."))
					})

					It("propogates the error", func() {
						_, err := execProcess.Wait()
						Expect(err).To(MatchError("Oh no."))
					})
				})
			})

			Context("when the user lookupper fails", func() {
				BeforeEach(func() {
					userLookupper.LookupReturns(nil, errors.New("user-lookup-failure"))
				})

				It("returns the error", func() {
					Expect(execErr).To(MatchError("user-lookup-failure"))
				})
			})

			Context("when getting the container PID fails", func() {
				BeforeEach(func() {
					containerManager.GetContainerPIDReturns(0, errors.New("get-container-pid-failure"))
				})

				It("returns the error", func() {
					Expect(execErr).To(MatchError("get-container-pid-failure"))
				})
			})

			Context("when bundleLoader returns an error", func() {
				BeforeEach(func() {
					bundleLoader.LoadReturns(goci.Bndl{}, errors.New("error-loading-bundle"))
				})

				It("returns the error", func() {
					Expect(execErr).To(MatchError("error-loading-bundle"))
				})
			})

			Context("when containerManager returns an error", func() {
				BeforeEach(func() {
					containerManager.ExecReturns(errors.New("error-execing"))
				})

				It("returns the error", func() {
					Expect(execErr).To(MatchError("error-execing"))
				})
			})

			Context("when a working directory is not specified", func() {
				BeforeEach(func() {
					processSpec.Dir = ""
				})

				It("sets the spec dir to the user home dir if no dir specified", func() {
					Expect(processBuilder.BuildProcessCallCount()).To(Equal(1))
					_, actualProcessSpec, _, _ := processBuilder.BuildProcessArgsForCall(0)
					Expect(actualProcessSpec.Dir).To(Equal("/home/alice"))
				})
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

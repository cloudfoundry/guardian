package runcontainerd_test

import (
	"bytes"
	"errors"
	"io"
	"syscall"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/event"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/runcontainerdfakes"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	"code.cloudfoundry.org/guardian/rundmc/users"
	"code.cloudfoundry.org/guardian/rundmc/users/usersfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	apievents "github.com/containerd/containerd/api/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Runcontainerd", func() {
	var (
		logger           lager.Logger
		containerManager *runcontainerdfakes.FakeContainerManager
		processManager   *runcontainerdfakes.FakeProcessManager
		runContainerd    *runcontainerd.RunContainerd
		execer           *runcontainerdfakes.FakeExecer
		statser          *runcontainerdfakes.FakeStatser
		processBuilder   *runcontainerdfakes.FakeProcessBuilder
		userLookupper    *usersfakes.FakeUserLookupper
		cgroupManager    *runcontainerdfakes.FakeCgroupManager
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-logger")
		containerManager = new(runcontainerdfakes.FakeContainerManager)
		processManager = new(runcontainerdfakes.FakeProcessManager)
		execer = new(runcontainerdfakes.FakeExecer)
		statser = new(runcontainerdfakes.FakeStatser)
		processBuilder = new(runcontainerdfakes.FakeProcessBuilder)
		userLookupper = new(usersfakes.FakeUserLookupper)
		cgroupManager = new(runcontainerdfakes.FakeCgroupManager)

		runContainerd = runcontainerd.New(containerManager, processManager, processBuilder, userLookupper, execer, statser, false, cgroupManager)
	})

	Describe("Create", func() {
		var (
			log    lager.Logger
			id     string
			bundle goci.Bndl

			stdin  io.Reader
			stdout io.Writer
			stderr io.Writer

			createErr error
		)

		BeforeEach(func() {
			log = lagertest.NewTestLogger("test-logger")
			id = "container-id"
			bundle = goci.Bndl{
				Spec: specs.Spec{
					Hostname: "test-hostname",
				},
			}

			stdin = new(bytes.Buffer)
			stdout = new(bytes.Buffer)
			stderr = new(bytes.Buffer)
		})

		JustBeforeEach(func() {
			createErr = runContainerd.Create(log, id, bundle, garden.ProcessIO{Stdin: stdin, Stdout: stdout, Stderr: stderr})
		})

		It("creates the container with the passed containerID", func() {
			Expect(containerManager.CreateCallCount()).To(Equal(1))
			_, actualID, _, _ := containerManager.CreateArgsForCall(0)
			Expect(actualID).To(Equal(id))
		})

		It("creates the container using the spec from the loaded bundle", func() {
			Expect(containerManager.CreateCallCount()).To(Equal(1))
			_, _, actualSpec, _ := containerManager.CreateArgsForCall(0)
			Expect(actualSpec).To(Equal(&bundle.Spec))
		})

		It("creates the container and attaches IO", func() {
			Expect(containerManager.CreateCallCount()).To(Equal(1))
			_, _, _, processIO := containerManager.CreateArgsForCall(0)

			actualStdin, actualStdout, actualStderr := processIO()
			Expect(actualStdin).To(BeIdenticalTo(stdin))
			Expect(actualStdout).To(BeIdenticalTo(stdout))
			Expect(actualStderr).To(BeIdenticalTo(stderr))
		})

		Context("when creating the container returns an error", func() {
			BeforeEach(func() {
				containerManager.CreateReturns(errors.New("EXPLODE"))
			})

			It("bubbles up that", func() {
				Expect(createErr).To(MatchError("EXPLODE"))
			})
		})

		Context("when using containerd for processes", func() {
			BeforeEach(func() {
				runContainerd = runcontainerd.New(containerManager, processManager, processBuilder, userLookupper, execer, statser, true, cgroupManager)
			})

			It("sets the container to use the memory hierarchy", func() {
				Expect(cgroupManager.SetUseMemoryHierarchyCallCount()).To(Equal(1))
				actualID := cgroupManager.SetUseMemoryHierarchyArgsForCall(0)
				Expect(actualID).To(Equal(id))
			})

			Context("when setting the container to use the memory hierarchy fails", func() {
				BeforeEach(func() {
					cgroupManager.SetUseMemoryHierarchyReturns(errors.New("NOPE"))
				})

				It("returns the error", func() {
					Expect(createErr).To(MatchError("NOPE"))
				})
			})
		})
	})

	Describe("Delete", func() {
		var deleteErr error

		JustBeforeEach(func() {
			deleteErr = runContainerd.Delete(nil, "container-id")
		})

		It("deletes the containerd task with the passed id", func() {
			Expect(containerManager.DeleteCallCount()).To(Equal(1))
			_, actualID := containerManager.DeleteArgsForCall(0)
			Expect(actualID).To(Equal("container-id"))
		})

		Context("when deleting a containerd task errors", func() {
			BeforeEach(func() {
				containerManager.DeleteReturns(errors.New("could not delete"))
			})

			It("bubbles up that error", func() {
				Expect(deleteErr).To(MatchError("could not delete"))
			})
		})
	})

	Describe("RemoveBundle", func() {
		var removeErr error

		JustBeforeEach(func() {
			removeErr = runContainerd.RemoveBundle(nil, "container-id")
		})

		It("deletes the containerd container with the passed id", func() {
			Expect(containerManager.RemoveBundleCallCount()).To(Equal(1))
			_, actualID := containerManager.RemoveBundleArgsForCall(0)
			Expect(actualID).To(Equal("container-id"))
		})

		Context("when deleting a containerd container errors", func() {
			BeforeEach(func() {
				containerManager.RemoveBundleReturns(errors.New("could not delete"))
			})

			It("bubbles up that error", func() {
				Expect(removeErr).To(MatchError("could not delete"))
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
			processSpec garden.ProcessSpec
			execErr     error
			bundle      goci.Bndl
			fakeProcess *gardenfakes.FakeProcess
			execProcess garden.Process
			processIO   garden.ProcessIO
		)

		BeforeEach(func() {
			fakeProcess = new(gardenfakes.FakeProcess)
			execer.ExecWithBndlReturns(fakeProcess, nil)
			containerID = "container-id"
			processSpec = garden.ProcessSpec{
				ID:   "test-process-id",
				User: "alice",
			}
			processBuilder.BuildProcessReturns(&specs.Process{
				Args: []string{"test-binary"},
			})
			bundle = goci.Bndl{Spec: specs.Spec{Hostname: "test-hostname"}}
			containerManager.SpecReturns(&bundle.Spec, nil)
			processIO = garden.ProcessIO{
				Stdin:  gbytes.NewBuffer(),
				Stdout: gbytes.NewBuffer(),
				Stderr: gbytes.NewBuffer(),
			}
		})

		JustBeforeEach(func() {
			execProcess, execErr = runContainerd.Exec(logger, containerID, processSpec, processIO)
		})

		It("gets the bundle", func() {
			Expect(containerManager.SpecCallCount()).To(Equal(1))
			_, actualContainerID := containerManager.SpecArgsForCall(0)
			Expect(actualContainerID).To(Equal(containerID))
		})

		It("delegates to execer", func() {
			Expect(execer.ExecWithBndlCallCount()).To(Equal(1))
			actualLogger, actualSandboxHandle, actualBundle, actualProcessSpec, actualIO := execer.ExecWithBndlArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualSandboxHandle).To(Equal(containerID))
			Expect(actualBundle).To(Equal(bundle))
			Expect(actualProcessSpec).To(Equal(processSpec))
			Expect(actualIO).To(Equal(processIO))
			Expect(execProcess).To(Equal(fakeProcess))
		})

		Context("when getting the bundle fails", func() {
			BeforeEach(func() {
				containerManager.SpecReturns(nil, errors.New("getting-bundle-failed"))
			})

			It("propagates the error", func() {
				Expect(execErr).To(MatchError("getting-bundle-failed"))
			})
		})

		Context("when the execer fails", func() {
			BeforeEach(func() {
				execer.ExecWithBndlReturns(nil, errors.New("execer-failed"))
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
				runContainerd = runcontainerd.New(containerManager, processManager, processBuilder, userLookupper, execer, statser, true, cgroupManager)
			})

			It("passes the logger through", func() {
				Expect(containerManager.ExecCallCount()).To(Equal(1))
				actualLogger, _, _, _, _ := containerManager.ExecArgsForCall(0)
				Expect(actualLogger).To(Equal(logger))
			})

			It("passes the io through", func() {
				Expect(containerManager.ExecCallCount()).To(Equal(1))
				_, _, _, _, actualIOFunc := containerManager.ExecArgsForCall(0)
				actualIn, actualOut, actualErr, _ := actualIOFunc()

				Expect(actualIn).To(Equal(processIO.Stdin))
				Expect(actualOut).To(Equal(processIO.Stdout))
				Expect(actualErr).To(Equal(processIO.Stderr))
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

			Context("when the binary does not exist in $PATH", func() {
				BeforeEach(func() {
					containerManager.ExecReturns(errors.New("OCI runtime exec failed: exec failed: container_linux.go:345: starting container process caused \"exec: potato: executable file not found in $PATH"))
				})

				It("returns a garden.ExecutableNotFoundError error", func() {
					Expect(execErr).To(BeAssignableToTypeOf(garden.ExecutableNotFoundError{}))
				})
			})

			Context("when the binary is filly qualified and does not exist", func() {
				BeforeEach(func() {
					containerManager.ExecReturns(errors.New("OCI runtime exec failed: exec failed: container_linux.go:345: starting container process caused \"exec: potato: stat potato: no such file or directory"))
				})

				It("returns a garden.ExecutableNotFoundError error", func() {
					Expect(execErr).To(BeAssignableToTypeOf(garden.ExecutableNotFoundError{}))
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

				It("converts and forwards signals to the process manager", func() {
					Expect(execProcess.Signal(garden.SignalTerminate)).To(Succeed())
					Expect(execProcess.Signal(garden.SignalKill)).To(Succeed())

					Expect(processManager.SignalCallCount()).To(Equal(2))
					_, firstActualContainerID, firstActualProcessID, firstActualSignal := processManager.SignalArgsForCall(0)
					Expect(firstActualContainerID).To(Equal("container-id"))
					Expect(firstActualProcessID).To(Equal("test-process-id"))
					Expect(firstActualSignal).To(Equal(syscall.SIGTERM))

					_, _, _, secondActualSignal := processManager.SignalArgsForCall(1)
					Expect(secondActualSignal).To(Equal(syscall.SIGKILL))
				})

				Context("when getting the bundle fails", func() {
					BeforeEach(func() {
						containerManager.SpecReturns(nil, errors.New("you-are-a-failure"))
					})

					It("propagates the error", func() {
						Expect(execErr).To(MatchError("you-are-a-failure"))
					})
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

				Context("when sending a signal that doesn't exist", func() {
					It("returns an appropriate error", func() {
						err := execProcess.Signal(-1)
						Expect(err).To(MatchError("Cannot convert garden signal -1 to syscall.Signal"))
					})
				})

				Context("when signalling the process fails", func() {
					BeforeEach(func() {
						processManager.SignalReturns(errors.New("EXPLODE"))
					})

					It("propagates error", func() {
						err := execProcess.Signal(garden.SignalTerminate)
						Expect(err).To(MatchError("EXPLODE"))
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
			attachProcess, attachError = runContainerd.Attach(logger, "some-id", "some-proc-id", io)
		})

		It("delegates to execer", func() {
			Expect(attachError).NotTo(HaveOccurred())
			Expect(attachProcess).To(BeIdenticalTo(process))

			Expect(execer.AttachCallCount()).To(Equal(1))
			actualLogger, actualContainerID, actualProcessID, actualIO := execer.AttachArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
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
			metrics    gardener.StatsContainerMetrics
			metricsErr error
		)

		BeforeEach(func() {
			statser.StatsReturns(gardener.StatsContainerMetrics{
				CPU: garden.ContainerCPUStat{
					Usage: 123,
				}}, nil)
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
				statser.StatsReturns(gardener.StatsContainerMetrics{}, errors.New("statser-failure"))
			})

			It("returns the statser error", func() {
				Expect(metricsErr).To(MatchError("statser-failure"))
			})
		})
	})

	Describe("Events", func() {
		var (
			eventsChannel <-chan event.Event
			eventsErr     error
			oomEvents     chan *apievents.TaskOOM
		)

		BeforeEach(func() {
			oomEvents = make(chan *apievents.TaskOOM)
			containerManager.OOMEventsReturnsOnCall(0, oomEvents)

		})

		JustBeforeEach(func() {
			eventsChannel, eventsErr = runContainerd.Events(logger)
		})

		AfterEach(func() {
			close(oomEvents)
		})

		It("succeeds", func() {
			Expect(eventsErr).NotTo(HaveOccurred())
		})

		It("reports OOM events", func() {
			Eventually(containerManager.OOMEventsCallCount, "10s").Should(Equal(1))
			actualLogger := containerManager.OOMEventsArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))

			var oomEvent event.Event
			oomEvents <- &apievents.TaskOOM{ContainerID: "my-container-id"}
			Eventually(eventsChannel).Should(Receive(&oomEvent))
			Expect(oomEvent.ContainerID).To(Equal("my-container-id"))
		})

		When("the events channel gets closed unexpectedly", func() {
			var (
				events      chan *apievents.TaskOOM
				eventsAgain chan *apievents.TaskOOM
			)

			BeforeEach(func() {
				events = make(chan *apievents.TaskOOM)
				eventsAgain = make(chan *apievents.TaskOOM)
				containerManager.OOMEventsReturnsOnCall(0, events)
				containerManager.OOMEventsReturnsOnCall(1, eventsAgain)
			})

			It("contiues to report OOM events", func() {
				close(events)
				Eventually(containerManager.OOMEventsCallCount, "10s").Should(Equal(2))

				var oomEvent event.Event
				eventsAgain <- &apievents.TaskOOM{ContainerID: "my-second-container-id"}
				Eventually(eventsChannel).Should(Receive(&oomEvent))
				Expect(oomEvent.ContainerID).To(Equal("my-second-container-id"))
			})

		})
	})

	Describe("BundleInfo", func() {
		var (
			bundlePath string
			bundle     goci.Bndl
			err        error
		)

		BeforeEach(func() {
			containerManager.SpecReturns(&specs.Spec{Version: "my-spec"}, nil)
		})

		JustBeforeEach(func() {
			bundlePath, bundle, err = runContainerd.BundleInfo(logger, "my-container")
		})

		It("returns the bundle for the specified container", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(bundlePath).To(BeEmpty())
			Expect(bundle.Spec.Version).To(Equal("my-spec"))

			Expect(containerManager.SpecCallCount()).To(Equal(1))
			actualLogger, actualHandle := containerManager.SpecArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualHandle).To(Equal("my-container"))
		})

		When("the container does not exist", func() {
			BeforeEach(func() {
				containerManager.SpecReturns(&specs.Spec{}, runcontainerd.ContainerNotFoundError{Handle: "my-container"})
			})

			It("returns a garden.ContainerNotFoundError", func() {
				Expect(err).To(Equal(garden.ContainerNotFoundError{Handle: "my-container"}))
			})
		})

		When("getting the container spec fails", func() {
			BeforeEach(func() {
				containerManager.SpecReturns(&specs.Spec{}, errors.New("spec-error"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("spec-error"))
			})
		})
	})

	Describe("BundleIDs", func() {
		var (
			bundleIDs []string
			err       error
		)

		BeforeEach(func() {
			containerManager.BundleIDsReturns([]string{"banana", "banana2"}, nil)
		})

		JustBeforeEach(func() {
			bundleIDs, err = runContainerd.BundleIDs()
		})

		It("returns the list of bundleIDs", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(bundleIDs).To(ConsistOf("banana", "banana2"))
		})

		When("getting the list of bundleIDs from the container manager fails", func() {
			BeforeEach(func() {
				containerManager.BundleIDsReturns(nil, errors.New("handles-error"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("handles-error"))
			})
		})
	})
})

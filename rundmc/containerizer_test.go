package rundmc_test

import (
	"bytes"
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/guardian/gardener"
	specpkg "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc"
	gardencgroups "code.cloudfoundry.org/guardian/rundmc/cgroups"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/event"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	fakes "code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Rundmc", func() {
	var (
		fakeDepot               *fakes.FakeDepot
		fakeBundleGenerator     *fakes.FakeBundleGenerator
		fakeOCIRuntime          *fakes.FakeOCIRuntime
		fakeNstarRunner         *fakes.FakeNstarRunner
		fakeProcessesStopper    *fakes.FakeProcessesStopper
		fakeEventStore          *fakes.FakeEventStore
		fakeStateStore          *fakes.FakeStateStore
		fakePeaCreator          *fakes.FakePeaCreator
		fakePeaUsernameResolver *fakes.FakePeaUsernameResolver
		fakeRuntimeStopper      *fakes.FakeRuntimeStopper
		fakeCPUCgrouper         *fakes.FakeCPUCgrouper

		logger        lager.Logger
		containerizer *rundmc.Containerizer
		bundle        goci.Bndl
	)

	BeforeEach(func() {
		fakeDepot = new(fakes.FakeDepot)
		fakeBundleGenerator = new(fakes.FakeBundleGenerator)
		fakeOCIRuntime = new(fakes.FakeOCIRuntime)
		fakeNstarRunner = new(fakes.FakeNstarRunner)
		fakeProcessesStopper = new(fakes.FakeProcessesStopper)
		fakeEventStore = new(fakes.FakeEventStore)
		fakeStateStore = new(fakes.FakeStateStore)
		fakePeaCreator = new(fakes.FakePeaCreator)
		fakePeaUsernameResolver = new(fakes.FakePeaUsernameResolver)
		fakeRuntimeStopper = new(fakes.FakeRuntimeStopper)
		fakeCPUCgrouper = new(fakes.FakeCPUCgrouper)
		logger = lagertest.NewTestLogger("test")

		bundle = goci.Bndl{Spec: specs.Spec{Version: "test-version"}}
		fakeBundleGenerator.GenerateReturns(bundle, nil)

		containerizer = rundmc.New(
			fakeDepot,
			fakeBundleGenerator,
			fakeOCIRuntime,
			fakeNstarRunner,
			fakeProcessesStopper,
			fakeEventStore,
			fakeStateStore,
			fakePeaCreator,
			fakePeaUsernameResolver,
			0,
			fakeRuntimeStopper,
			fakeCPUCgrouper,
		)
	})

	Describe("Create", func() {
		It("should generate an oci bundle", func() {
			spec := specpkg.DesiredContainerSpec{
				Handle:     "exuberant!",
				BaseConfig: specs.Spec{Root: &specs.Root{}},
			}
			Expect(containerizer.Create(logger, spec)).To(Succeed())

			Expect(fakeBundleGenerator.GenerateCallCount()).To(Equal(1))

			actualSpec := fakeBundleGenerator.GenerateArgsForCall(0)
			Expect(actualSpec).To(Equal(spec))
		})

		It("should create a bad CPU cgroup", func() {
			Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
				Handle:     "exuberant!",
				BaseConfig: specs.Spec{Root: &specs.Root{}},
			})).To(Succeed())

			Expect(fakeCPUCgrouper.PrepareCgroupsCallCount()).To(Equal(1))
			actualHandle := fakeCPUCgrouper.PrepareCgroupsArgsForCall(0)
			Expect(actualHandle).To(Equal("exuberant!"))
		})

		It("should create a container with the given id", func() {
			Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{
				Handle:     "exuberant!",
				BaseConfig: specs.Spec{Root: &specs.Root{}},
			})).To(Succeed())

			Expect(fakeOCIRuntime.CreateCallCount()).To(Equal(1))

			_, actualId, actualBundle, _ := fakeOCIRuntime.CreateArgsForCall(0)
			Expect(actualBundle).To(Equal(bundle))
			Expect(actualId).To(Equal("exuberant!"))
		})

		Context("when the bundle generation fails", func() {
			BeforeEach(func() {
				fakeBundleGenerator.GenerateReturns(goci.Bndl{}, errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{})).To(MatchError("banana"))
			})
		})

		Context("when creating the bad cgroup fails", func() {
			BeforeEach(func() {
				fakeCPUCgrouper.PrepareCgroupsReturns(errors.New("BOOHOO"))
			})

			It("should propagate the error", func() {
				Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{})).To(MatchError("BOOHOO"))
			})

		})
		Context("when the container creation fails", func() {
			BeforeEach(func() {
				fakeOCIRuntime.CreateReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(containerizer.Create(logger, specpkg.DesiredContainerSpec{})).To(MatchError("banana"))
			})
		})
	})

	Describe("Run", func() {
		It("should ask the execer to exec a process in the container", func() {
			_, err := containerizer.Run(logger, "some-handle", garden.ProcessSpec{Path: "hello"}, garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeOCIRuntime.ExecCallCount()).To(Equal(1))

			_, id, spec, _ := fakeOCIRuntime.ExecArgsForCall(0)
			Expect(id).To(Equal("some-handle"))
			Expect(spec.Path).To(Equal("hello"))
		})

		Context("when process has no image", func() {
			It("doesn't create a pea", func() {
				_, err := containerizer.Run(logger, "some-handle", garden.ProcessSpec{Path: "hello"}, garden.ProcessIO{})
				Expect(err).NotTo(HaveOccurred())
				Expect(fakePeaCreator.CreatePeaCallCount()).To(Equal(0))
			})

			Context("when bind mounts are provided", func() {
				It("returns an error", func() {
					_, err := containerizer.Run(logger, "some-handle",
						garden.ProcessSpec{
							Path: "hello",
							BindMounts: []garden.BindMount{
								garden.BindMount{
									SrcPath: "src",
									DstPath: "dst",
								},
							},
						},
						garden.ProcessIO{})
					Expect(err).To(MatchError("Running a process with bind mounts and no image provided is not allowed"))
				})
			})
		})

		Context("when process has an image", func() {
			var (
				processSpec garden.ProcessSpec
				pio         garden.ProcessIO
			)

			BeforeEach(func() {
				processSpec = garden.ProcessSpec{Image: garden.ImageRef{URI: "some-uri"}}
				pio = garden.ProcessIO{Stdout: bytes.NewBufferString("some-idiosyncratic buffer")}
			})

			It("creates a pea", func() {
				_, err := containerizer.Run(logger, "some-handle", processSpec, pio)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakePeaCreator.CreatePeaCallCount()).To(Equal(1))
				_, actualProcessSpec, actualProcessIO, actualHandle := fakePeaCreator.CreatePeaArgsForCall(0)
				Expect(actualProcessSpec).To(Equal(processSpec))
				Expect(actualHandle).To(Equal("some-handle"))
				Expect(actualProcessIO).To(Equal(pio))
			})

			It("returns process from pea creator", func() {
				fakeProcess := new(gardenfakes.FakeProcess)
				fakeProcess.IDReturns("some-id")
				fakePeaCreator.CreatePeaReturns(fakeProcess, nil)
				process, err := containerizer.Run(logger, "some-handle", processSpec, pio)
				Expect(process.ID()).To(Equal("some-id"))
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("Username resolving", func() {
				Context("when user is not specified", func() {
					It("does not try to resolve the user", func() {
						_, err := containerizer.Run(logger, "some-handle", processSpec, pio)
						Expect(err).NotTo(HaveOccurred())
						Expect(fakePeaUsernameResolver.ResolveUserCallCount()).To(Equal(0))
					})
				})

				Context("when user is specified as uid:gid", func() {
					BeforeEach(func() {
						processSpec.User = "1:2"
					})

					It("does not try to resolve the user", func() {
						_, err := containerizer.Run(logger, "some-handle", processSpec, pio)
						Expect(err).NotTo(HaveOccurred())
						Expect(fakePeaUsernameResolver.ResolveUserCallCount()).To(Equal(0))
					})
				})

				Context("when user is specified as username", func() {
					BeforeEach(func() {
						processSpec.User = "foobar"
					})

					It("resolves username to uid:gid", func() {
						fakePeaUsernameResolver.ResolveUserReturns(1, 2, nil)

						_, err := containerizer.Run(logger, "some-handle", processSpec, pio)
						Expect(err).NotTo(HaveOccurred())

						Expect(fakePeaUsernameResolver.ResolveUserCallCount()).To(Equal(1))
						_, _, _, resolverInputUsername := fakePeaUsernameResolver.ResolveUserArgsForCall(0)
						Expect(resolverInputUsername).To(Equal("foobar"))

						Expect(fakePeaCreator.CreatePeaCallCount()).To(Equal(1))
						_, createdPeaProcessSpec, _, _ := fakePeaCreator.CreatePeaArgsForCall(0)
						Expect(createdPeaProcessSpec.User).To(Equal("1:2"))
					})
				})
			})
		})
	})

	Describe("Attach", func() {
		It("should ask the execer to attach a process in the container", func() {
			_, err := containerizer.Attach(logger, "some-handle", "123", garden.ProcessIO{})
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeOCIRuntime.AttachCallCount()).To(Equal(1))

			_, id, processId, _ := fakeOCIRuntime.AttachArgsForCall(0)
			Expect(id).To(Equal("some-handle"))
			Expect(processId).To(Equal("123"))
		})
	})

	Describe("StreamIn", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeOCIRuntime.StateReturns(rundmc.State{
				Pid: 12,
			}, nil)

			someStream := gbytes.NewBuffer()
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{
				Path:      "some-path",
				User:      "some-user",
				TarStream: someStream,
			})).To(Succeed())

			_, pid, path, user, stream := fakeNstarRunner.StreamInArgsForCall(0)
			Expect(pid).To(Equal(12))
			Expect(path).To(Equal("some-path"))
			Expect(user).To(Equal("some-user"))
			Expect(stream).To(Equal(someStream))
		})

		It("returns an error if the PID cannot be found", func() {
			fakeOCIRuntime.StateReturns(rundmc.State{}, errors.New("pid not found"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: pid not found for container"))
		})

		It("returns the error if nstar fails", func() {
			fakeNstarRunner.StreamInReturns(errors.New("failed"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: nstar: failed"))
		})
	})

	Describe("StreamOut", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeOCIRuntime.StateReturns(rundmc.State{
				Pid: 12,
			}, nil)

			fakeNstarRunner.StreamOutReturns(os.Stdin, nil)

			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{
				Path: "some-path",
				User: "some-user",
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(tarStream).To(Equal(os.Stdin))

			_, pid, path, user := fakeNstarRunner.StreamOutArgsForCall(0)
			Expect(pid).To(Equal(12))
			Expect(path).To(Equal("some-path"))
			Expect(user).To(Equal("some-user"))
		})

		It("returns an error if the PID cannot be found", func() {
			fakeOCIRuntime.StateReturns(rundmc.State{}, errors.New("pid not found"))
			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{})

			Expect(tarStream).To(BeNil())
			Expect(err).To(MatchError("stream-out: pid not found for container"))
		})

		It("returns the error if nstar fails", func() {
			fakeNstarRunner.StreamOutReturns(nil, errors.New("failed"))
			tarStream, err := containerizer.StreamOut(logger, "some-handle", garden.StreamOutSpec{})

			Expect(tarStream).To(BeNil())
			Expect(err).To(MatchError("stream-out: nstar: failed"))
		})
	})

	Describe("Stop", func() {
		var (
			cgroupPathArg string
			exceptionsArg []int
			killArg       bool
		)

		Context("when the stop succeeds", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(rundmc.State{
					Pid: 1234,
				}, nil)

				Expect(containerizer.Stop(logger, "some-handle", true)).To(Succeed())
				Expect(fakeProcessesStopper.StopAllCallCount()).To(Equal(1))

				_, cgroupPathArg, exceptionsArg, killArg = fakeProcessesStopper.StopAllArgsForCall(0)
			})

			It("asks to stop all processes in the processes's cgroup", func() {
				Expect(cgroupPathArg).To(Equal("some-handle"))
				Expect(killArg).To(Equal(true))
			})

			It("asks to not stop the pid of the init process", func() {
				Expect(exceptionsArg).To(ConsistOf(1234))
			})

			It("transitions the stored state", func() {
				Expect(fakeStateStore.StoreStoppedCallCount()).To(Equal(1))
				handle := fakeStateStore.StoreStoppedArgsForCall(0)
				Expect(handle).To(Equal("some-handle"))
			})
		})

		Context("when stopping container processes fails", func() {
			BeforeEach(func() {
				fakeProcessesStopper.StopAllReturns(errors.New("boom"))
			})

			It("does not transition to the stopped state", func() {
				Expect(containerizer.Stop(logger, "some-handle", true)).To(MatchError(ContainSubstring("boom")))
				Expect(fakeStateStore.StoreStoppedCallCount()).To(Equal(0))
			})
		})

		Context("when getting runc's state fails", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(rundmc.State{}, errors.New("boom"))
			})

			It("does not stop the processes", func() {
				Expect(fakeProcessesStopper.StopAllCallCount()).To(Equal(0))
			})

			It("does not transition to the stopped state", func() {
				Expect(containerizer.Stop(logger, "some-handle", true)).To(MatchError(ContainSubstring("boom")))
				Expect(fakeStateStore.StoreStoppedCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Destroy", func() {
		It("delegates to the OCI runtime", func() {
			Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
			Expect(fakeOCIRuntime.DeleteCallCount()).To(Equal(1))
		})

		It("destroys the bad cgroup", func() {
			Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
			Expect(fakeCPUCgrouper.CleanupCgroupsCallCount()).To(Equal(1))
			Expect(fakeCPUCgrouper.CleanupCgroupsArgsForCall(0)).To(Equal("some-handle"))
		})

		Context("when the runtime fails to destroy", func() {
			BeforeEach(func() {
				fakeOCIRuntime.DeleteReturns(errors.New("pid not found"))
			})

			It("propagates the error back", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(MatchError("pid not found"))
			})
		})

		Context("when deleting the bad cgroup fails", func() {
			BeforeEach(func() {
				fakeCPUCgrouper.CleanupCgroupsReturns(errors.New("POOH"))
			})

			It("propagates the error back", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(MatchError("POOH"))
			})
		})

	})

	Describe("RemoveBundle", func() {
		It("removes the bundle", func() {
			Expect(containerizer.RemoveBundle(logger, "some-handle")).To(Succeed())

			// TODO: this should be removed once containerd processes are on by default
			Expect(fakeDepot.DestroyCallCount()).To(Equal(1))
			_, handle := fakeDepot.DestroyArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))

			Expect(fakeOCIRuntime.RemoveBundleCallCount()).To(Equal(1))
			_, handle = fakeOCIRuntime.RemoveBundleArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		Context("when removing bundle fails", func() {
			BeforeEach(func() {
				fakeOCIRuntime.RemoveBundleReturns(errors.New("destroy failed"))
			})

			It("returns an error", func() {
				Expect(containerizer.RemoveBundle(logger, "some-handle")).To(MatchError(ContainSubstring("destroy failed")))
			})
		})
	})

	Describe("Info", func() {
		var namespaces []specs.LinuxNamespace
		var resources *specs.LinuxResources

		BeforeEach(func() {
			namespaces = []specs.LinuxNamespace{}
			fakeOCIRuntime.StateReturns(rundmc.State{Pid: 42}, nil)

			var limit int64 = 10
			var shares uint64 = 20
			resources = &specs.LinuxResources{
				Memory: &specs.LinuxMemory{
					Limit: &limit,
				},
				CPU: &specs.LinuxCPU{
					Shares: &shares,
				},
			}

		})

		JustBeforeEach(func() {
			fakeOCIRuntime.BundleInfoStub = func(_ lager.Logger, handle string) (string, goci.Bndl, error) {
				if handle != "some-handle" {
					return "", goci.Bundle(), errors.New("cannot find bundle")
				}

				return "/path/to/" + handle, goci.Bndl{
					Spec: specs.Spec{
						Root: &specs.Root{},
						Linux: &specs.Linux{
							Namespaces: namespaces,
							Resources:  resources,
						},
					},
				}, nil
			}
		})

		It("should return the ActualContainerSpec with the correct bundlePath", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.BundlePath).To(Equal("/path/to/some-handle"))
		})

		It("should return the ActualContainerSpec with the correct CPU limits", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			//lint:ignore SA1019 - we still specify this to make the deprecated logic work until we get rid of the code in garden
			Expect(actualSpec.Limits.CPU.LimitInShares).To(BeEquivalentTo(20))
		})

		It("should return the ActualContainerSpec with the correct memory limits", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Limits.Memory.LimitInBytes).To(BeEquivalentTo(10))
		})

		It("should return the ActualContainerSpec with the correct pid", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Pid).To(Equal(42))
		})

		Context("when loading up the bundle info fails", func() {
			It("should return the error", func() {
				fakeOCIRuntime.BundleInfoReturns("", goci.Bndl{}, errors.New("bundle-info-error"))
				_, err := containerizer.Info(logger, "some-handle")
				Expect(err).To(MatchError("bundle-info-error"))
			})
		})

		Context("when the bundle has no resources", func() {
			BeforeEach(func() {
				resources = nil
			})

			It("doesn't fail", func() {
				actualSpec, err := containerizer.Info(logger, "some-handle")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualSpec.Limits.Memory.LimitInBytes).To(BeEquivalentTo(0))
				//lint:ignore SA1019 - we still specify this to make the deprecated logic work until we get rid of the code in garden
				Expect(actualSpec.Limits.CPU.LimitInShares).To(BeEquivalentTo(0))
			})
		})

		It("should return any events from the event store", func() {
			fakeEventStore.EventsReturns([]string{
				"potato",
				"fire",
			})

			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Events).To(Equal([]string{
				"potato",
				"fire",
			}))
		})

		It("should return the stopped state from the property manager", func() {
			fakeStateStore.IsStoppedReturns(true)

			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Stopped).To(Equal(true))
		})

		It("should return the ActualContainerSpec with privileged by default", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.Privileged).To(BeTrue())
		})

		Context("when namespaces contains the user namespace", func() {
			BeforeEach(func() {
				namespaces = append(namespaces, specs.LinuxNamespace{Type: specs.UserNamespace})
			})

			It("should return the ActualContainerSpec with privileged", func() {
				actualSpec, err := containerizer.Info(logger, "some-handle")
				Expect(err).NotTo(HaveOccurred())
				Expect(actualSpec.Privileged).To(BeFalse())
			})
		})

		Context("when retrieving the State from the runtime errors", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StateReturns(rundmc.State{}, errors.New("error-fetching-state"))
			})

			It("returns the error", func() {
				_, err := containerizer.Info(logger, "some-handle")
				Expect(err).To(MatchError("error-fetching-state"))
			})
		})
	})

	Describe("Metrics", func() {
		JustBeforeEach(func() {
			memoryLimit := int64(100)
			fakeOCIRuntime.BundleInfoReturns("", goci.Bndl{
				Spec: specs.Spec{
					Root: &specs.Root{},
					Linux: &specs.Linux{
						Resources: &specs.LinuxResources{
							Memory: &specs.LinuxMemory{
								Limit: &memoryLimit,
							},
						},
					},
				},
			}, nil)
		})

		It("returns the CPU metrics reported by cgrouper", func() {
			containerStats := gardener.StatsContainerMetrics{
				CPU: garden.ContainerCPUStat{
					Usage:  1,
					User:   2,
					System: 3,
				},
			}

			cpuStats := garden.ContainerCPUStat{
				Usage:  2,
				User:   4,
				System: 6,
			}

			expectedMetrics := gardener.ActualContainerMetrics{
				StatsContainerMetrics: gardener.StatsContainerMetrics{
					CPU: garden.ContainerCPUStat{
						Usage:  2,
						User:   4,
						System: 6,
					},
				},
			}
			fakeOCIRuntime.StatsReturns(containerStats, nil)
			fakeCPUCgrouper.ReadTotalCgroupUsageReturns(cpuStats, nil)
			Expect(containerizer.Metrics(logger, "foo")).To(Equal(expectedMetrics))
			handle, stats := fakeCPUCgrouper.ReadTotalCgroupUsageArgsForCall(0)
			Expect(handle).To(Equal("foo"))
			Expect(stats).To(Equal(containerStats.CPU))
		})

		Context("when cpu entitlement per share is defined", func() {
			var entitlementPerSharePercent float64

			BeforeEach(func() {
				entitlementPerSharePercent = 0.01
				containerizer = rundmc.New(
					fakeDepot,
					fakeBundleGenerator,
					fakeOCIRuntime,
					fakeNstarRunner,
					fakeProcessesStopper,
					fakeEventStore,
					fakeStateStore,
					fakePeaCreator,
					fakePeaUsernameResolver,
					entitlementPerSharePercent,
					fakeRuntimeStopper,
					fakeCPUCgrouper,
				)
			})

			It("return the CPU entitlement", func() {
				cpuShares := uint64(100)
				fakeOCIRuntime.BundleInfoReturns("", goci.Bundle().WithCPUShares(specs.LinuxCPU{Shares: &cpuShares}), nil)

				containerAge := time.Second * 5
				fakeOCIRuntime.StatsReturns(gardener.StatsContainerMetrics{
					Age: containerAge,
				}, nil)

				actualMetrics, err := containerizer.Metrics(logger, "foo")
				Expect(err).NotTo(HaveOccurred())
				if gardencgroups.IsCgroup2UnifiedMode() {
					expectedEntitlement := uint64(float64(cpuShares) * (entitlementPerSharePercent / 100) * float64(containerAge))
					Expect(actualMetrics.CPUEntitlement).To(BeNumerically("~", expectedEntitlement, 1_000_001))

				} else {
					expectedEntitlement := uint64(float64(cpuShares) * (entitlementPerSharePercent / 100) * float64(containerAge))
					Expect(actualMetrics.CPUEntitlement).To(Equal(expectedEntitlement))
				}
			})

			Context("when peas metrics are requested", func() {
				BeforeEach(func() {
					fakeOCIRuntime.BundleInfoReturns("", goci.Bndl{}, depot.ErrDoesNotExist)
				})

				It("does not return CPU entitlement", func() {
					actualMetrics, err := containerizer.Metrics(logger, "foo")
					Expect(err).NotTo(HaveOccurred())
					Expect(actualMetrics.CPUEntitlement).To(Equal(uint64(0)))
				})
			})
		})

		Context("when container fails to provide stats", func() {
			BeforeEach(func() {
				fakeOCIRuntime.StatsReturns(gardener.StatsContainerMetrics{}, errors.New("banana"))
			})

			It("should return the error", func() {
				_, err := containerizer.Metrics(logger, "foo")
				Expect(err).To(MatchError("banana"))
			})
		})

		Context("when the cpu cgrouper fails to provide bad cgroup usage", func() {
			BeforeEach(func() {
				fakeCPUCgrouper.ReadTotalCgroupUsageReturns(garden.ContainerCPUStat{}, errors.New("potato"))
			})

			It("should return the error", func() {
				_, err := containerizer.Metrics(logger, "foo")
				Expect(err).To(MatchError("potato"))
			})
		})

		Context("when the bad cgroup does not exist (an ancient container)", func() {
			var runtimeStats gardener.StatsContainerMetrics

			BeforeEach(func() {
				fakeCPUCgrouper.ReadTotalCgroupUsageReturns(garden.ContainerCPUStat{}, os.ErrNotExist)

				runtimeStats = gardener.StatsContainerMetrics{
					CPU: garden.ContainerCPUStat{
						Usage:  1,
						User:   2,
						System: 3,
					},
				}
				fakeOCIRuntime.StatsReturns(runtimeStats, nil)
			})

			It("returns runtime stats only", func() {
				Expect(containerizer.Metrics(logger, "foo")).To(Equal(gardener.ActualContainerMetrics{StatsContainerMetrics: runtimeStats}))
			})
		})

		Context("when the bad cgroup does not exist and error type is not recognized", func() {
			var runtimeStats gardener.StatsContainerMetrics

			BeforeEach(func() {
				err := errors.New("error while statting cgroup v2: [open group.procs: no such file or directory]")
				fakeCPUCgrouper.ReadTotalCgroupUsageReturns(garden.ContainerCPUStat{}, err)

				runtimeStats = gardener.StatsContainerMetrics{
					CPU: garden.ContainerCPUStat{
						Usage:  1,
						User:   2,
						System: 3,
					},
				}
				fakeOCIRuntime.StatsReturns(runtimeStats, nil)
			})

			It("returns runtime stats only", func() {
				Expect(containerizer.Metrics(logger, "foo")).To(Equal(gardener.ActualContainerMetrics{StatsContainerMetrics: runtimeStats}))
			})
		})
	})

	Describe("Handles", func() {
		BeforeEach(func() {
			fakeOCIRuntime.ContainerHandlesReturns([]string{"banana", "banana2"}, nil)
		})

		It("should return the handles", func() {
			Expect(containerizer.Handles()).To(ConsistOf("banana", "banana2"))
		})

		Context("when the runtime returns an error", func() {
			BeforeEach(func() {
				fakeOCIRuntime.ContainerHandlesReturns([]string{}, errors.New("handles-error"))
			})

			It("should return the error", func() {
				_, err := containerizer.Handles()
				Expect(err).To(MatchError("handles-error"))
			})
		})
	})

	Describe("WatchRuntimeEvents", func() {
		var watchErr error

		JustBeforeEach(func() {
			watchErr = containerizer.WatchRuntimeEvents(logger)
		})

		Context("when the runtime publishes an event", func() {
			BeforeEach(func() {
				events := make(chan event.Event)
				go func() {
					events <- event.Event{Message: "1"}
					events <- event.Event{Message: "2"}
				}()
				fakeOCIRuntime.EventsReturns(events, nil)
			})

			It("forwards the event to the event store", func() {
				Expect(watchErr).NotTo(HaveOccurred())
				Eventually(fakeEventStore.OnEventCallCount).Should(Equal(2))

				_, msg := fakeEventStore.OnEventArgsForCall(0)
				Expect(msg).To(Equal("1"))

				_, msg = fakeEventStore.OnEventArgsForCall(1)
				Expect(msg).To(Equal("2"))
			})
		})

		Context("when getting the events channel errors", func() {
			BeforeEach(func() {
				fakeOCIRuntime.EventsReturns(nil, errors.New("boom"))
			})

			It("errors", func() {
				Expect(watchErr).To(MatchError("boom"))
			})
		})
	})

	Describe("Shutdown", func() {
		var shutdownError error

		JustBeforeEach(func() {
			shutdownError = containerizer.Shutdown()
		})

		It("stops the runtime", func() {
			Expect(shutdownError).NotTo(HaveOccurred())
			Expect(fakeRuntimeStopper.StopCallCount()).To(Equal(1))
		})

		Context("when stopping the runtime fails", func() {
			BeforeEach(func() {
				fakeRuntimeStopper.StopReturns(errors.New("stop-rt-err"))
			})

			It("returns the error", func() {
				Expect(shutdownError).To(MatchError("stop-rt-err"))
			})
		})
	})
})

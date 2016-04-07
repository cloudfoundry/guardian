package rundmc_test

import (
	"errors"
	"os"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/opencontainers/specs/specs-go"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Rundmc", func() {
	var (
		fakeDepot           *fakes.FakeDepot
		fakeBundler         *fakes.FakeBundleGenerator
		fakeBundleLoader    *fakes.FakeBundleLoader
		fakeContainerRunner *fakes.FakeBundleRunner
		fakeNstarRunner     *fakes.FakeNstarRunner
		fakeEventStore      *fakes.FakeEventStore

		logger        lager.Logger
		containerizer *rundmc.Containerizer
	)

	BeforeEach(func() {
		fakeDepot = new(fakes.FakeDepot)
		fakeContainerRunner = new(fakes.FakeBundleRunner)
		fakeBundler = new(fakes.FakeBundleGenerator)
		fakeBundleLoader = new(fakes.FakeBundleLoader)
		fakeNstarRunner = new(fakes.FakeNstarRunner)
		fakeEventStore = new(fakes.FakeEventStore)
		logger = lagertest.NewTestLogger("test")

		fakeDepot.LookupStub = func(_ lager.Logger, handle string) (string, error) {
			return "/path/to/" + handle, nil
		}

		containerizer = rundmc.New(fakeDepot, fakeBundler, fakeContainerRunner, fakeBundleLoader, fakeNstarRunner, fakeEventStore)
	})

	Describe("Create", func() {
		It("should ask the depot to create a container", func() {
			var returnedBundle *goci.Bndl
			fakeBundler.GenerateStub = func(spec gardener.DesiredContainerSpec) *goci.Bndl {
				return returnedBundle
			}

			containerizer.Create(logger, gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})

			Expect(fakeDepot.CreateCallCount()).To(Equal(1))

			_, handle, bundle := fakeDepot.CreateArgsForCall(0)
			Expect(handle).To(Equal("exuberant!"))
			Expect(bundle).To(Equal(returnedBundle))
		})

		Context("when creating the depot directory fails", func() {
			It("returns an error", func() {
				fakeDepot.CreateReturns(errors.New("blam"))
				Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{
					Handle: "exuberant!",
				})).NotTo(Succeed())
			})
		})

		It("should start a container in the created directory", func() {
			Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})).To(Succeed())

			Expect(fakeContainerRunner.StartCallCount()).To(Equal(1))

			_, path, id, _ := fakeContainerRunner.StartArgsForCall(0)
			Expect(path).To(Equal("/path/to/exuberant!"))
			Expect(id).To(Equal("exuberant!"))
		})

		It("should prepare the root file system", func() {
			Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{
				Handle: "exuberant!",
			})).To(Succeed())

		})

		Context("when the container fails to start", func() {
			BeforeEach(func() {
				fakeContainerRunner.StartReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{})).NotTo(Succeed())
			})
		})

		It("should watch for events in a goroutine", func() {
			fakeContainerRunner.WatchEventsStub = func(_ lager.Logger, _ string, _ runrunc.EventsNotifier) error {
				time.Sleep(10 * time.Second)
				return nil
			}

			created := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				Expect(containerizer.Create(logger, gardener.DesiredContainerSpec{Handle: "some-container"})).To(Succeed())
				close(created)
			}()

			select {
			case <-time.After(2 * time.Second):
				Fail("WatchEvents should be called in a goroutine")
			case <-created:
			}

			Eventually(fakeContainerRunner.WatchEventsCallCount).Should(Equal(1))

			_, handle, eventsNotifier := fakeContainerRunner.WatchEventsArgsForCall(0)
			Expect(handle).To(Equal("some-container"))
			Expect(eventsNotifier).To(Equal(fakeEventStore))
		})
	})

	Describe("Run", func() {
		It("should ask the execer to exec a process in the container", func() {
			containerizer.Run(logger, "some-handle", garden.ProcessSpec{Path: "hello"}, garden.ProcessIO{})
			Expect(fakeContainerRunner.ExecCallCount()).To(Equal(1))

			_, path, id, spec, _ := fakeContainerRunner.ExecArgsForCall(0)
			Expect(path).To(Equal("/path/to/some-handle"))
			Expect(id).To(Equal("some-handle"))
			Expect(spec.Path).To(Equal("hello"))
		})

		Context("when looking up the container fails", func() {
			It("returns an error", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				_, err := containerizer.Run(logger, "some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(err).To(HaveOccurred())
			})

			It("does not attempt to exec the process", func() {
				fakeDepot.LookupReturns("", errors.New("blam"))
				containerizer.Run(logger, "some-handle", garden.ProcessSpec{}, garden.ProcessIO{})
				Expect(fakeContainerRunner.StartCallCount()).To(Equal(0))
			})
		})
	})

	Describe("StreamIn", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeContainerRunner.StateReturns(runrunc.State{
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
			fakeContainerRunner.StateReturns(runrunc.State{}, errors.New("pid not found"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: pid not found for container"))
		})

		It("returns the error if nstar fails", func() {
			fakeNstarRunner.StreamInReturns(errors.New("failed"))
			Expect(containerizer.StreamIn(logger, "some-handle", garden.StreamInSpec{})).To(MatchError("stream-in: nstar: failed"))
		})
	})

	Describe("StreamOut", func() {
		It("should execute the NSTar command with the container PID", func() {
			fakeContainerRunner.StateReturns(runrunc.State{
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
			fakeContainerRunner.StateReturns(runrunc.State{}, errors.New("pid not found"))
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

	Describe("destroy", func() {
		Context("when getting state fails", func() {
			BeforeEach(func() {
				fakeContainerRunner.StateReturns(runrunc.State{}, errors.New("pid not found"))
			})

			It("should NOT run kill", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeContainerRunner.KillCallCount()).To(Equal(0))
			})

			It("should destroy the depot directory", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeDepot.DestroyCallCount()).To(Equal(1))
				Expect(arg2(fakeDepot.DestroyArgsForCall(0))).To(Equal("some-handle"))
			})
		})

		Context("when state is running", func() {
			BeforeEach(func() {
				fakeContainerRunner.StateReturns(runrunc.State{
					Status: "running",
				}, nil)
			})

			It("should run kill", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeContainerRunner.KillCallCount()).To(Equal(1))
				Expect(arg2(fakeContainerRunner.KillArgsForCall(0))).To(Equal("some-handle"))
			})

			Context("when kill succeeds", func() {
				It("destroys the depot directory", func() {
					Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
					Expect(fakeDepot.DestroyCallCount()).To(Equal(1))
					Expect(arg2(fakeDepot.DestroyArgsForCall(0))).To(Equal("some-handle"))
				})
			})

			Context("when kill fails", func() {
				It("does not destroy the depot directory", func() {
					fakeContainerRunner.KillReturns(errors.New("killing is wrong"))
					containerizer.Destroy(logger, "some-handle")
					Expect(fakeDepot.DestroyCallCount()).To(Equal(0))
				})
			})
		})

		Context("when state is not running", func() {
			BeforeEach(func() {
				fakeContainerRunner.StateReturns(runrunc.State{
					Status: "potato",
				}, nil)
			})

			It("should not run kill", func() {
				Expect(containerizer.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeContainerRunner.KillCallCount()).To(Equal(0))
			})
		})
	})

	Describe("Info", func() {
		It("should return the ActualContainerSpec with the correct bundlePath", func() {
			actualSpec, err := containerizer.Info(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(actualSpec.BundlePath).To(Equal("/path/to/some-handle"))
		})

		Context("when looking up the bundle path fails", func() {
			It("should return the error", func() {
				fakeDepot.LookupReturns("", errors.New("spiderman-error"))
				_, err := containerizer.Info(logger, "some-handle")
				Expect(err).To(MatchError("spiderman-error"))
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
	})

	Describe("Metrics", func() {
		It("returns the CPU metrics", func() {
			metrics := gardener.ActualContainerMetrics{
				CPU: garden.ContainerCPUStat{
					Usage:  1,
					User:   2,
					System: 3,
				},
			}

			fakeContainerRunner.StatsReturns(metrics, nil)
			Expect(containerizer.Metrics(logger, "foo")).To(Equal(metrics))
		})

		Context("when container fails to provide stats", func() {
			BeforeEach(func() {
				fakeContainerRunner.StatsReturns(gardener.ActualContainerMetrics{}, errors.New("banana"))
			})

			It("should return the error", func() {
				_, err := containerizer.Metrics(logger, "foo")
				Expect(err).To(MatchError("banana"))
			})
		})
	})

	Describe("CPULimit", func() {
		BeforeEach(func() {
			fakeBundleLoader.LoadStub = func(bundlePath string) (*goci.Bndl, error) {
				if bundlePath != "/path/to/some-handle" {
					return nil, errors.New("cannot find bundle")
				}

				var shares uint64 = 10
				return &goci.Bndl{
					Spec: specs.Spec{
						Linux: specs.Linux{
							Resources: &specs.Resources{
								CPU: &specs.CPU{
									Shares: &shares,
								},
							},
						},
					},
				}, nil
			}
		})

		It("should return the limit", func() {
			cpuLimit, err := containerizer.CPULimit(logger, "some-handle")
			Expect(err).NotTo(HaveOccurred())
			Expect(cpuLimit.LimitInShares).To(BeEquivalentTo(10))
		})

		Context("when the container does not exist", func() {
			BeforeEach(func() {
				fakeDepot.LookupReturns("", errors.New("aaa"))
			})

			It("returns an error", func() {
				_, err := containerizer.CPULimit(logger, "some-handle")
				Expect(err).To(MatchError("looking up container: aaa"))
			})
		})

		Context("when the bundle is not found", func() {
			BeforeEach(func() {
				fakeBundleLoader.LoadReturns(nil, errors.New("banana"))
			})

			It("returns an error", func() {
				_, err := containerizer.CPULimit(logger, "some-handle")
				Expect(err).To(MatchError("loading bundle: banana"))
			})
		})
	})

	Describe("handles", func() {
		Context("when handles exist", func() {
			BeforeEach(func() {
				fakeDepot.HandlesReturns([]string{"banana", "banana2"}, nil)
			})

			It("should return the handles", func() {
				Expect(containerizer.Handles()).To(ConsistOf("banana", "banana2"))
			})
		})

		Context("when the depot returns an error", func() {
			testErr := errors.New("spiderman error")

			BeforeEach(func() {
				fakeDepot.HandlesReturns([]string{}, testErr)
			})

			It("should return the error", func() {
				_, err := containerizer.Handles()
				Expect(err).To(MatchError(testErr))
			})
		})
	})
})

func arg2(_ lager.Logger, i interface{}) interface{} {
	return i
}

package gardener_test

import (
	"errors"
	"fmt"
	"net"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	fakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/lager/v3"
	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Gardener", func() {
	var (
		networker              *fakes.FakeNetworker
		volumizer              *fakes.FakeVolumizer
		containerizer          *fakes.FakeContainerizer
		uidGenerator           *fakes.FakeUidGenerator
		bulkStarter            *fakes.FakeBulkStarter
		peaCleaner             *fakes.FakePeaCleaner
		sysinfoProvider        *fakes.FakeSysInfoProvider
		propertyManager        *fakes.FakePropertyManager
		restorer               *fakes.FakeRestorer
		sleeper                *fakes.FakeSleeper
		networkMetricsProvider *fakes.FakeContainerNetworkMetricsProvider

		logger *lagertest.TestLogger

		gdnr *gardener.Gardener
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		containerizer = new(fakes.FakeContainerizer)
		uidGenerator = new(fakes.FakeUidGenerator)
		bulkStarter = new(fakes.FakeBulkStarter)
		peaCleaner = new(fakes.FakePeaCleaner)
		networker = new(fakes.FakeNetworker)
		volumizer = new(fakes.FakeVolumizer)
		sysinfoProvider = new(fakes.FakeSysInfoProvider)
		propertyManager = new(fakes.FakePropertyManager)
		restorer = new(fakes.FakeRestorer)
		sleeper = new(fakes.FakeSleeper)
		networkMetricsProvider = new(fakes.FakeContainerNetworkMetricsProvider)

		propertyManager.GetReturns("", true)
		networker.SetupBindMountsReturns([]garden.BindMount{}, nil)
		volumizer.CreateReturns(specs.Spec{Root: &specs.Root{Path: ""}}, nil)
		containerizer.HandlesReturns([]string{"some-handle"}, nil)
		containerizer.InfoReturns(spec.ActualContainerSpec{Pid: 470, RootFSPath: "rootfs"}, nil)

		gdnr = gardener.New(
			uidGenerator,
			bulkStarter,
			sysinfoProvider,
			networker,
			volumizer,
			containerizer,
			propertyManager,
			restorer,
			peaCleaner,
			logger,
			0,
			false,
			networkMetricsProvider,
		)
		gdnr.Sleep = sleeper.Spy
	})

	Describe("creating a container", func() {
		ItDestroysEverything := func() {
			It("should clean up the networking configuration", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
				Expect(err).To(HaveOccurred())
				Expect(networker.DestroyCallCount()).To(Equal(1))
				_, handle := networker.DestroyArgsForCall(0)
				Expect(handle).To(Equal("poor-banana"))
			})

			It("should clean up any created volumes", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
				Expect(err).To(HaveOccurred())
				Expect(volumizer.DestroyCallCount()).To(Equal(1))
				_, handle := volumizer.DestroyArgsForCall(0)
				Expect(handle).To(Equal("poor-banana"))
			})

			It("should destroy any container state", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
				Expect(err).To(HaveOccurred())
				Expect(containerizer.DestroyCallCount()).To(Equal(1))
				_, handle := containerizer.DestroyArgsForCall(0)
				Expect(handle).To(Equal("poor-banana"))
			})

			It("should remove the bundle", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
				Expect(err).To(HaveOccurred())
				Expect(containerizer.RemoveBundleCallCount()).To(Equal(1))
				_, handle := containerizer.RemoveBundleArgsForCall(0)
				Expect(handle).To(Equal("poor-banana"))
			})
		}

		It("assigns a random handle to the container", func() {
			uidGenerator.GenerateReturns("generated-handle")

			_, err := gdnr.Create(garden.ContainerSpec{})

			Expect(err).NotTo(HaveOccurred())
			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.Handle).To(Equal("generated-handle"))
		})

		It("assigns the hostname to be the same as the random handle", func() {
			uidGenerator.GenerateReturns("generated-handle")

			_, err := gdnr.Create(garden.ContainerSpec{})

			Expect(err).NotTo(HaveOccurred())
			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.Handle).To(Equal("generated-handle"))
			Expect(spec.Hostname).To(Equal(spec.Handle))
		})

		Context("when a handle is specified", func() {
			It("assigns the handle to the container", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "handle"})
				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.Handle).To(Equal("handle"))
			})
		})

		It("runs the graph cleanup", func() {
			_, err := gdnr.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(volumizer.GCCallCount()).To(Equal(1))
		})

		Context("when the graph cleanup fails", func() {
			It("does NOT return the error", func() {
				volumizer.GCReturns(errors.New("graph-cleanup-fail"))
				_, err := gdnr.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		It("passes the ContainerSpec to the Volumizer", func() {
			spec := garden.ContainerSpec{Handle: "some-ctr"}
			_, err := gdnr.Create(spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumizer.CreateCallCount()).To(Equal(1))
			_, actualContainerSpec := volumizer.CreateArgsForCall(0)
			Expect(actualContainerSpec).To(Equal(spec))
		})

		It("fails to create privileged containers", func() {
			_, err := gdnr.Create(garden.ContainerSpec{
				Privileged: true,
			})
			Expect(err).To(MatchError("privileged container creation is disabled"))
		})

		Context("when privileged containers are allowed", func() {
			BeforeEach(func() {
				gdnr.AllowPrivilgedContainers = true
			})

			It("can create privileged containers", func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					Privileged: true,
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when volume creation fails", func() {
			BeforeEach(func() {
				volumizer.CreateReturns(specs.Spec{}, errors.New("booom!"))
			})

			It("returns an error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(MatchError("booom!"))
			})

			It("retains container properties", func() {
				gdnr.Create(garden.ContainerSpec{Handle: "bob", Properties: garden.Properties{"owner": "some-owner"}})
				Expect(propertyManager.SetCallCount()).To(Equal(1))
				handle, name, value := propertyManager.SetArgsForCall(0)
				Expect(handle).To(Equal("bob"))
				Expect(name).To(Equal("owner"))
				Expect(value).To(Equal("some-owner"))
			})

			It("should not call the containerizer", func() {
				gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(containerizer.CreateCallCount()).To(Equal(0))
			})

			ItDestroysEverything()
		})

		It("sets up bind mounts through the networker", func() {
			gdnr.AllowPrivilgedContainers = true

			volumizer.CreateReturns(specs.Spec{
				Root: &specs.Root{Path: "/rootfs/path"},
			}, nil)

			networker.SetupBindMountsReturns([]garden.BindMount{
				{SrcPath: "from-networker"},
			}, nil)

			_, err := gdnr.Create(garden.ContainerSpec{
				Handle:     "some-ctr",
				Privileged: true,
				BindMounts: []garden.BindMount{
					{SrcPath: "original"},
				},
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(networker.SetupBindMountsCallCount()).To(Equal(1))
			_, actualHandle, actualPrivileged, actualRootfsPath := networker.SetupBindMountsArgsForCall(0)
			Expect(actualHandle).To(Equal("some-ctr"))
			Expect(actualPrivileged).To(BeTrue())
			Expect(actualRootfsPath).To(Equal("/rootfs/path"))

			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, actualDesiredSpec := containerizer.CreateArgsForCall(0)
			Expect(actualDesiredSpec.BindMounts).To(Equal([]garden.BindMount{
				{SrcPath: "original"},
				{SrcPath: "from-networker"},
			}))
		})

		Context("when setting up bind mounts fails", func() {
			BeforeEach(func() {
				networker.SetupBindMountsReturns(nil, errors.New("failed"))
			})

			It("returns the error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "some-ctr"})
				Expect(err).To(MatchError("failed"))
			})

			It("sets the container properties", func() {
				gdnr.Create(garden.ContainerSpec{Handle: "bob", Properties: garden.Properties{"owner": "some-owner"}})
				Expect(propertyManager.SetCallCount()).To(Equal(1))
				handle, name, value := propertyManager.SetArgsForCall(0)
				Expect(handle).To(Equal("bob"))
				Expect(name).To(Equal("owner"))
				Expect(value).To(Equal("some-owner"))
			})
		})

		It("asks the containerizer to create a container", func() {
			_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
			Expect(err).NotTo(HaveOccurred())

			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.Handle).To(Equal("bob"))
		})

		It("sets the handle as the container hostname", func() {
			_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
			Expect(err).NotTo(HaveOccurred())

			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.Hostname).To(Equal("bob"))
		})

		Context("when the containerizer fails to create the container", func() {
			BeforeEach(func() {
				containerizer.CreateReturns(errors.New("failed to create the banana"))
			})

			It("should return an error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					Handle: "poor-banana",
				})
				Expect(err).To(HaveOccurred())
			})

			It("sets container properties", func() {
				gdnr.Create(garden.ContainerSpec{Handle: "bob", Properties: garden.Properties{"owner": "some-owner"}})
				Expect(propertyManager.SetCallCount()).To(Equal(1))
				handle, name, value := propertyManager.SetArgsForCall(0)
				Expect(handle).To(Equal("bob"))
				Expect(name).To(Equal("owner"))
				Expect(value).To(Equal("some-owner"))
			})

			ItDestroysEverything()

			It("logs the underlying error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(HaveOccurred())
				Eventually(logger).Should(gbytes.Say("failed to create the banana"))
			})

			Context("when any of the destroy operations fails", func() {
				BeforeEach(func() {
					volumizer.DestroyReturns(errors.New("failed to destroy the banana volume"))
				})

				It("does not destroy the bundle (so that the container can be looked up)", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
					Expect(err).To(HaveOccurred())
					Expect(containerizer.RemoveBundleCallCount()).To(BeZero())
				})

				It("should clean up the networking configuration", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
					Expect(err).To(HaveOccurred())
					Expect(networker.DestroyCallCount()).To(Equal(1))
					_, handle := networker.DestroyArgsForCall(0)
					Expect(handle).To(Equal("poor-banana"))
				})

				It("should clean up any created volumes", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
					Expect(err).To(HaveOccurred())
					Expect(volumizer.DestroyCallCount()).To(Equal(1))
					_, handle := volumizer.DestroyArgsForCall(0)
					Expect(handle).To(Equal("poor-banana"))
				})

				It("should destroy any container state", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
					Expect(err).To(HaveOccurred())
					Expect(containerizer.DestroyCallCount()).To(Equal(1))
					_, handle := containerizer.DestroyArgsForCall(0)
					Expect(handle).To(Equal("poor-banana"))
				})

				It("sets container properties", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana", Properties: garden.Properties{"owner": "some-owner"}})
					Expect(err).To(HaveOccurred())
					Expect(propertyManager.SetCallCount()).To(Equal(1))
					handle, name, value := propertyManager.SetArgsForCall(0)
					Expect(handle).To(Equal("poor-banana"))
					Expect(name).To(Equal("owner"))
					Expect(value).To(Equal("some-owner"))
				})
			})

			Context("when network destroy operation fails", func() {
				BeforeEach(func() {
					networker.DestroyReturns(errors.New("failed to destroy the banana network"))
				})

				It("should not destroy the network state metadata", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
					Expect(err).To(HaveOccurred())
					Expect(propertyManager.DestroyKeySpaceCallCount()).To(BeZero())
				})

				It("does not destroy the bundle (so that the container can be looked up)", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana"})
					Expect(err).To(HaveOccurred())
					Expect(containerizer.RemoveBundleCallCount()).To(BeZero())
				})

				It("sets container properties", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "poor-banana", Properties: garden.Properties{"owner": "some-owner"}})
					Expect(err).To(HaveOccurred())
					Expect(propertyManager.SetCallCount()).To(Equal(1))
					handle, name, value := propertyManager.SetArgsForCall(0)
					Expect(handle).To(Equal("poor-banana"))
					Expect(name).To(Equal("owner"))
					Expect(value).To(Equal("some-owner"))
				})
			})
		})

		Describe("ContainerSpec limits", func() {
			var spec garden.ContainerSpec

			BeforeEach(func() {
				spec.Limits.Disk.Scope = garden.DiskLimitScopeTotal
				spec.Limits.Disk.ByteHard = 10 * 1024 * 1024
			})

			It("calls the containerizer with the disk limit", func() {
				_, err := gdnr.Create(spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, actualDesiredContainerSpec := containerizer.CreateArgsForCall(0)
				Expect(actualDesiredContainerSpec.Limits).To(Equal(garden.Limits{
					Disk: garden.DiskLimits{
						Scope:    garden.DiskLimitScopeTotal,
						ByteHard: 10 * 1024 * 1024,
					},
				}))
			})
		})

		It("should ask the networker to configure the network", func() {
			containerizer.InfoReturns(spec.ActualContainerSpec{
				Pid:        42,
				BundlePath: "bndl",
			}, nil)
			_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
			Expect(err).NotTo(HaveOccurred())

			Expect(networker.NetworkCallCount()).To(Equal(1))
			_, spec, pid := networker.NetworkArgsForCall(0)
			Expect(spec).To(Equal(garden.ContainerSpec{
				Handle: "bob",
			}))
			Expect(pid).To(Equal(42))
		})

		Context("when container info cannot be retrieved", func() {
			It("errors", func() {
				containerizer.InfoReturns(spec.ActualContainerSpec{}, errors.New("boom"))

				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(MatchError("boom"))

			})
		})

		Context("when container info returns pid = 0", func() {
			BeforeEach(func() {
				containerizer.InfoReturns(spec.ActualContainerSpec{Pid: 0}, nil)
			})

			It("errors", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when networker fails to configure network", func() {
			It("errors", func() {
				networker.NetworkReturns(errors.New("network-failed"))

				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(MatchError("network-failed"))
			})

			It("sets container properties", func() {
				gdnr.Create(garden.ContainerSpec{Handle: "bob", Properties: garden.Properties{"owner": "some-owner"}})
				Expect(propertyManager.SetCallCount()).To(Equal(2)) // TODO chnage to 1 once we stop failure for non-healtcheck
				handle, name, value := propertyManager.SetArgsForCall(0)
				Expect(handle).To(Equal("bob"))
				Expect(name).To(Equal("owner"))
				Expect(value).To(Equal("some-owner"))
			})
		})

		Context("when a grace time is specified", func() {
			It("sets the grace time via the property manager", func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					Handle:    "something",
					GraceTime: time.Minute,
				})
				Expect(err).NotTo(HaveOccurred())

				handle, name, value := propertyManager.SetArgsForCall(0)
				Expect(handle).To(Equal("something"))
				Expect(name).To(Equal(gardener.GraceTimeKey))
				Expect(value).To(Equal(fmt.Sprintf("%d", time.Minute)))
			})
		})

		It("passes base config to containerizer", func() {
			runtimeConfig := specs.Spec{Version: "some-idiosyncratic-version", Root: &specs.Root{Path: ""}}
			volumizer.CreateReturns(runtimeConfig, nil)

			_, err := gdnr.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.BaseConfig).To(Equal(runtimeConfig))
		})

		It("passes env to containerizer", func() {
			_, err := gdnr.Create(garden.ContainerSpec{Env: []string{"FOO=bar"}})
			Expect(err).NotTo(HaveOccurred())

			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.Env).To(Equal([]string{"FOO=bar"}))
		})

		Context("when passed a handle that already exists", func() {
			var (
				containerSpec garden.ContainerSpec
			)

			BeforeEach(func() {
				containerizer.HandlesReturns([]string{"duplicate-banana"}, nil)
				containerSpec = garden.ContainerSpec{Handle: "duplicate-banana"}
			})

			It("returns a useful error message", func() {
				_, err := gdnr.Create(containerSpec)
				Expect(err).To(MatchError("Handle 'duplicate-banana' already in use"))
			})

			It("doesn't return a container", func() {
				container, _ := gdnr.Create(containerSpec)
				Expect(container).To(BeNil())
			})
		})

		Describe("MaxContainers", func() {
			BeforeEach(func() {
				containerizer.HandlesReturns([]string{"cake1", "cake2", "cake3"}, nil)
			})

			Context("when MaxContainers = 0", func() {
				It("succeeds", func() {
					_, err := gdnr.Create(garden.ContainerSpec{})
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when MaxContainers > 0", func() {
				BeforeEach(func() {
					gdnr.MaxContainers = 3
				})

				It("returns an error", func() {
					_, err := gdnr.Create(garden.ContainerSpec{})
					Expect(err).To(MatchError("max containers reached"))
				})
			})
		})

		Context("when containerizer.Handles() returns an error", func() {
			BeforeEach(func() {
				containerizer.HandlesReturns(nil, errors.New("error-fetching-handles"))
			})

			It("forwards the error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{})
				Expect(err).To(MatchError("error-fetching-handles"))
			})
		})

		Context("when properties are specified", func() {
			var startingProperties garden.Properties

			BeforeEach(func() {
				startingProperties = garden.Properties{
					"thingy": "thing",
					"blingy": "bling",
				}
			})

			It("sets every property on the container", func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					Handle:     "something",
					Properties: startingProperties,
				})
				Expect(err).NotTo(HaveOccurred())

				var allProps = make(map[string]string)
				for i := 0; i < 2; i++ {
					handle, name, value := propertyManager.SetArgsForCall(i)
					Expect(handle).To(Equal("something"))
					allProps[name] = value
				}

				Expect(allProps).To(Equal(map[string]string{
					"blingy": "bling",
					"thingy": "thing",
				}))
			})
		})

		It("sets the container state to created", func() {
			_, err := gdnr.Create(garden.ContainerSpec{
				Handle: "something",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(propertyManager.SetCallCount()).To(Equal(1))
			handle, name, value := propertyManager.SetArgsForCall(0)
			Expect(handle).To(Equal("something"))
			Expect(name).To(Equal("garden.state"))
			Expect(value).To(Equal("created"))
		})

		Context("when bind mounts are specified", func() {
			It("generates a proper mount spec", func() {
				bindMounts := []garden.BindMount{
					{
						SrcPath: "src",
						DstPath: "dst",
					},
				}

				_, err := gdnr.Create(garden.ContainerSpec{
					BindMounts: bindMounts,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.BindMounts).To(Equal(bindMounts))
			})
		})

		It("returns the container that Lookup would return", func() {
			c, err := gdnr.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())

			d, err := gdnr.Lookup(c.Handle())
			Expect(err).NotTo(HaveOccurred())
			Expect(c).To(Equal(d))
		})

		Context("when creating privileged containers is not permitted, and a privileged container is requested", func() {
			It("returns an error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Privileged: true})
				Expect(err).To(MatchError("privileged container creation is disabled"))
			})

			It("does not try to provision a volume", func() {
				gdnr.Create(garden.ContainerSpec{Privileged: true})
				Expect(volumizer.CreateCallCount()).To(Equal(0))
			})
		})
	})

	Context("when having a container", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("banana")
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("running a process in a container", func() {
			It("asks the containerizer to run the process", func() {
				origSpec := garden.ProcessSpec{Path: "ripe"}
				origIO := garden.ProcessIO{
					Stdout: gbytes.NewBuffer(),
				}
				_, err := container.Run(origSpec, origIO)
				Expect(err).ToNot(HaveOccurred())

				Expect(containerizer.RunCallCount()).To(Equal(1))
				_, id, spec, io := containerizer.RunArgsForCall(0)
				Expect(id).To(Equal("banana"))
				Expect(spec).To(Equal(origSpec))
				Expect(io).To(Equal(origIO))
			})

			Context("when the containerizer fails to run a process", func() {
				BeforeEach(func() {
					containerizer.RunReturns(nil, errors.New("lost my banana"))
				})

				It("returns the error", func() {
					_, err := container.Run(garden.ProcessSpec{}, garden.ProcessIO{})
					Expect(err).To(MatchError("lost my banana"))
				})
			})
		})

		Describe("attaching to an existing process in a container", func() {
			It("asks the containerizer to attach to the process", func() {
				origIO := garden.ProcessIO{
					Stdout: gbytes.NewBuffer(),
				}
				_, err := container.Attach("123", origIO)
				Expect(err).ToNot(HaveOccurred())

				Expect(containerizer.AttachCallCount()).To(Equal(1))
				_, id, processId, io := containerizer.AttachArgsForCall(0)
				Expect(id).To(Equal("banana"))
				Expect(processId).To(Equal("123"))
				Expect(io).To(Equal(origIO))
			})

			Context("when the containerizer fails to attach to a process", func() {
				BeforeEach(func() {
					containerizer.AttachReturns(nil, errors.New("lost my banana"))
				})

				It("returns the error", func() {
					_, err := container.Attach("123", garden.ProcessIO{})
					Expect(err).To(MatchError("lost my banana"))
				})
			})
		})

		Describe("streaming files in to the container", func() {
			It("asks the containerizer to stream in the tar stream", func() {
				spec := garden.StreamInSpec{Path: "potato", User: "chef", TarStream: gbytes.NewBuffer()}
				Expect(container.StreamIn(spec)).To(Succeed())

				_, handle, specArg := containerizer.StreamInArgsForCall(0)
				Expect(handle).To(Equal("banana"))
				Expect(specArg).To(Equal(spec))
			})
		})

		Describe("streaming files outside the container", func() {
			It("asks the containerizer to stream out the files", func() {
				spec := garden.StreamOutSpec{Path: "potato", User: "chef"}
				_, err := container.StreamOut(spec)
				Expect(err).To(Succeed())

				_, handle, specArg := containerizer.StreamOutArgsForCall(0)
				Expect(handle).To(Equal("banana"))
				Expect(specArg).To(Equal(spec))
			})
		})

		Describe("NetIn", func() {
			const (
				externalPort  uint32 = 8888
				contianerPort uint32 = 8080
			)

			It("asks the netwoker to forward the correct ports", func() {
				_, _, err := container.NetIn(externalPort, contianerPort)

				Expect(err).NotTo(HaveOccurred())
				Expect(networker.NetInCallCount()).To(Equal(1))

				actualLogger, actualHandle, actualExtPort, actualContainerPort := networker.NetInArgsForCall(0)
				Expect(actualLogger).To(Equal(logger))
				Expect(actualHandle).To(Equal(container.Handle()))
				Expect(actualExtPort).To(Equal(externalPort))
				Expect(actualContainerPort).To(Equal(contianerPort))
			})

			Context("when networker returns an error", func() {
				It("returns the error", func() {
					networker.NetInReturns(uint32(0), uint32(0), fmt.Errorf("error"))

					_, _, err := container.NetIn(externalPort, contianerPort)

					Expect(err).To(MatchError("error"))
				})
			})
		})

		Describe("NetOut", func() {
			var rule garden.NetOutRule

			BeforeEach(func() {
				rule = garden.NetOutRule{
					Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("8.2.3.4"))},
					Ports:    []garden.PortRange{garden.PortRangeFromPort(9321)},
				}
			})

			It("asks the networker to apply the provided netout rule", func() {
				Expect(container.NetOut(rule)).To(Succeed())
				Expect(networker.NetOutCallCount()).To(Equal(1))

				_, handle, actualRule := networker.NetOutArgsForCall(0)
				Expect(handle).To(Equal("banana"))
				Expect(actualRule).To(Equal(rule))
			})

			Context("when networker returns an error", func() {
				It("return the error", func() {
					networker.NetOutReturns(fmt.Errorf("banana republic"))
					Expect(container.NetOut(rule)).To(MatchError("banana republic"))
				})
			})
		})

		Describe("BulkNetOut", func() {
			var rules []garden.NetOutRule

			BeforeEach(func() {
				rules = []garden.NetOutRule{
					{
						Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("8.2.3.4"))},
						Ports:    []garden.PortRange{garden.PortRangeFromPort(9321)},
					},
					{
						Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("6.1.2.3"))},
						Ports:    []garden.PortRange{garden.PortRangeFromPort(7654)},
					},
				}
			})

			It("asks the networker to apply the provided netout rule", func() {
				Expect(container.BulkNetOut(rules)).To(Succeed())
				Expect(networker.BulkNetOutCallCount()).To(Equal(1))

				_, handle, actualRules := networker.BulkNetOutArgsForCall(0)
				Expect(handle).To(Equal("banana"))
				Expect(actualRules).To(ConsistOf(rules))
			})

			Context("when networker returns an error", func() {
				It("return the error", func() {
					networker.BulkNetOutReturns(fmt.Errorf("banana republic"))
					Expect(container.BulkNetOut(rules)).To(MatchError("banana republic"))
				})
			})
		})
	})

	Describe("starting up gardener", func() {
		BeforeEach(func() {
			containers := []string{"container1", "container2"}
			containerizer.HandlesReturns(containers, nil)
		})

		It("starts watching runtime events", func() {
			Expect(gdnr.Start()).To(Succeed())

			Expect(containerizer.WatchRuntimeEventsCallCount()).To(Equal(1))
		})

		Context("when watching runtime events fails", func() {
			BeforeEach(func() {
				containerizer.WatchRuntimeEventsReturns(errors.New("boom"))
			})

			It("returns the error", func() {
				Expect(gdnr.Start()).To(MatchError(ContainSubstring("boom")))
			})
		})

		It("calls the bulk starter", func() {
			Expect(gdnr.Start()).To(Succeed())

			Expect(bulkStarter.StartAllCallCount()).To(Equal(1))
		})

		Context("when the bulk starter fails", func() {
			BeforeEach(func() {
				bulkStarter.StartAllReturns(errors.New("boom"))
			})

			It("returns the error", func() {
				Expect(gdnr.Start()).To(MatchError(ContainSubstring("boom")))
			})

			It("does not restore the containers", func() {
				Expect(gdnr.Start()).NotTo(Succeed())
				Expect(restorer.RestoreCallCount()).To(Equal(0))
			})
		})

		It("should restore the containers", func() {
			Expect(gdnr.Start()).To(Succeed())
			Expect(restorer.RestoreCallCount()).To(Equal(1))
			_, handles := restorer.RestoreArgsForCall(0)
			Expect(handles).To(Equal([]string{"container1", "container2"}))
		})

		It("should blow up containers that couldn't restore", func() {
			restorer.RestoreReturns([]string{"container2"})
			Expect(gdnr.Start()).To(Succeed())
			Expect(containerizer.DestroyCallCount()).To(Equal(1))
			_, handle := containerizer.DestroyArgsForCall(0)
			Expect(handle).To(Equal("container2"))
		})

		It("should return the error when it failes to get a list of handles", func() {
			containerizer.HandlesReturns([]string{}, errors.New("banana"))
			Expect(gdnr.Start()).To(MatchError("cleanup: banana"))
		})

		It("should cleanup peas", func() {
			Expect(gdnr.Start()).To(Succeed())
			Expect(peaCleaner.CleanAllCallCount()).To(Equal(1))
		})

		Context("when the bulk starter fails", func() {
			BeforeEach(func() {
				peaCleaner.CleanAllReturns(errors.New("bam"))
			})

			It("returns the error", func() {
				Expect(gdnr.Start()).To(MatchError(ContainSubstring("bam")))
			})
		})
	})

	Describe("listing containers", func() {
		BeforeEach(func() {
			containerizer.HandlesReturns([]string{"banana", "banana2", "cola"}, nil)
		})

		itOnlyMatchesFullyCreatedContainers := func(props garden.Properties) {
			It("only matches fully created containers", func() {
				_, err := gdnr.Containers(props)
				Expect(err).NotTo(HaveOccurred())

				_, props := propertyManager.MatchesAllArgsForCall(0)
				Expect(props).To(HaveKeyWithValue("garden.state", "created"))
			})
		}

		Context("when passed nil properties to match against", func() {
			itOnlyMatchesFullyCreatedContainers(nil)
		})

		Context("when garden.Properties are passed", func() {
			props := garden.Properties{"somename": "somevalue"}

			It("only returns matching containers", func() {
				propertyManager.MatchesAllStub = func(handle string, props garden.Properties) bool {
					return handle != "banana"
				}

				c, err := gdnr.Containers(props)
				Expect(err).NotTo(HaveOccurred())
				Expect(c).To(HaveLen(2))
				Expect(c[0].Handle()).To(Equal("banana2"))
				Expect(c[1].Handle()).To(Equal("cola"))
			})

			itOnlyMatchesFullyCreatedContainers(props)
		})

		Context("when garden state is set to all", func() {
			It("returns all containers including non-created containers", func() {
				props := garden.Properties{"garden.state": "all"}
				_, err := gdnr.Containers(props)
				Expect(err).NotTo(HaveOccurred())

				_, props = propertyManager.MatchesAllArgsForCall(0)
				Expect(props).ToNot(HaveKey("garden.state"))
			})
		})
	})

	Context("when no containers exist", func() {
		BeforeEach(func() {
			containerizer.HandlesReturns([]string{}, nil)
		})

		It("should return an empty list", func() {
			containers, err := gdnr.Containers(garden.Properties{})
			Expect(err).NotTo(HaveOccurred())

			Expect(containers).To(BeEmpty())
		})
	})

	Context("when the containerizer returns an error", func() {
		testErr := errors.New("failure")

		BeforeEach(func() {
			containerizer.HandlesReturns([]string{}, testErr)
		})

		It("should return an error", func() {
			_, err := gdnr.Containers(garden.Properties{})
			Expect(err).To(MatchError(testErr))
		})
	})

	Describe("stopping a container", func() {
		It("asks the containerizer to stop all processes", func() {
			container, err := gdnr.Lookup("banana")
			Expect(err).NotTo(HaveOccurred())

			Expect(container.Stop(true)).To(Succeed())
			Expect(containerizer.StopCallCount()).To(Equal(1))

			_, handle, kill := containerizer.StopArgsForCall(0)
			Expect(handle).To(Equal("banana"))
			Expect(kill).To(Equal(true))
		})
	})

	Describe("Destroy", func() {
		It("returns garden.ContainerNotFoundError if the container handle isn't in the depot", func() {
			containerizer.HandlesReturns([]string{}, nil)
			Expect(gdnr.Destroy("cake!")).To(MatchError(garden.ContainerNotFoundError{Handle: "cake!"}))
		})

		It("asks the containerizer to destroy the container", func() {
			Expect(gdnr.Destroy("some-handle")).To(Succeed())
			Expect(containerizer.DestroyCallCount()).To(Equal(1))
			_, handle := containerizer.DestroyArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		It("asks the networker to destroy the container network", func() {
			gdnr.Destroy("some-handle")
			Expect(networker.DestroyCallCount()).To(Equal(1))
			_, handleToDestroy := networker.DestroyArgsForCall(0)
			Expect(handleToDestroy).To(Equal("some-handle"))
		})

		It("asks the volume creator to destroy the container rootfs", func() {
			gdnr.Destroy("some-handle")
			Expect(volumizer.DestroyCallCount()).To(Equal(1))
			_, handleToDestroy := volumizer.DestroyArgsForCall(0)
			Expect(handleToDestroy).To(Equal("some-handle"))
		})

		It("should destroy the key space of the property manager", func() {
			gdnr.Destroy("some-handle")

			Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(1))
			Expect(propertyManager.DestroyKeySpaceArgsForCall(0)).To(Equal("some-handle"))
		})

		It("asks the containerizer to remove the bundle from the depot", func() {
			Expect(gdnr.Destroy("some-handle")).To(Succeed())
			_, handle := containerizer.RemoveBundleArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		Context("when containerizer fails to destroy the container", func() {
			BeforeEach(func() {
				containerizer.DestroyReturns(errors.New("containerized deletion failed"))
			})

			It("return the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError(ContainSubstring("containerized deletion failed")))
			})

			It("should not destroy the bundle", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(containerizer.RemoveBundleCallCount()).To(Equal(0))
			})

			It("should not destroy the container keyspace in the propertyManager", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(0))
			})
		})

		Context("when containerizer fails to remove the bundle from the depot", func() {
			BeforeEach(func() {
				containerizer.DestroyReturns(errors.New("containerized deletion failed"))
			})

			It("return the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError(ContainSubstring("containerized deletion failed")))
			})
		})

		Context("when network deletion fails", func() {
			BeforeEach(func() {
				networker.DestroyReturns(errors.New("network deletion failed"))
			})

			It("returns the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError(ContainSubstring("network deletion failed")))
			})

			It("should not destroy the bundle", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(containerizer.RemoveBundleCallCount()).To(Equal(0))
			})

			It("should not destroy the key space of the property manager", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(0))
			})
		})

		Context("when destroying the rootfs fails", func() {
			BeforeEach(func() {
				volumizer.DestroyReturns(errors.New("rootfs deletion failed"))
			})

			It("returns the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError(ContainSubstring("rootfs deletion failed")))
			})

			It("should not destroy the bundle", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(containerizer.RemoveBundleCallCount()).To(Equal(0))
			})

			It("should not destroy the key space of the property manager", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(0))
			})
		})

		Context("when destroying key space fails", func() {
			BeforeEach(func() {
				propertyManager.DestroyKeySpaceReturns(errors.New("key space destruction failed"))
			})

			It("returns the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError(ContainSubstring("key space destruction failed")))
			})
		})

		Context("when containerizer fails to remove the bundle from the depot", func() {
			BeforeEach(func() {
				containerizer.RemoveBundleReturns(errors.New("bundle removal failed"))
			})

			It("returns the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError("bundle removal failed"))
			})
		})
	})

	Describe("Cleanup", func() {
		var cleanupErr error

		BeforeEach(func() {
			restorer.RestoreReturns([]string{"unrestorable-handle-1", "unrestorable-handle-2"})
		})

		JustBeforeEach(func() {
			cleanupErr = gdnr.Cleanup(logger)
		})

		It("succeeds", func() {
			Expect(cleanupErr).NotTo(HaveOccurred())
		})

		It("cleans all peas", func() {
			Expect(peaCleaner.CleanAllCallCount()).To(Equal(1))
			actualLogger := peaCleaner.CleanAllArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
		})

		It("tries to restore all handles", func() {
			Expect(restorer.RestoreCallCount()).To(Equal(1))
			actualLogger, actualRestoredHandles := restorer.RestoreArgsForCall(0)
			Expect(actualLogger).To(Equal(logger))
			Expect(actualRestoredHandles).To(Equal([]string{"some-handle"}))
		})

		It("destroys every handle that couldn't be restored", func() {
			handles := []string{"unrestorable-handle-1", "unrestorable-handle-2"}

			Expect(containerizer.DestroyCallCount()).To(Equal(len(handles)))
			actualHandles := []string{}
			for i, _ := range handles {
				_, actualHandle := containerizer.DestroyArgsForCall(i)
				actualHandles = append(actualHandles, actualHandle)
			}
			Expect(actualHandles).To(ConsistOf(handles))

			actualHandles = []string{}
			Expect(networker.DestroyCallCount()).To(Equal(len(handles)))
			for i, _ := range handles {
				_, actualHandle := networker.DestroyArgsForCall(i)
				actualHandles = append(actualHandles, actualHandle)
			}
			Expect(actualHandles).To(ConsistOf(handles))

			actualHandles = []string{}
			Expect(volumizer.DestroyCallCount()).To(Equal(len(handles)))
			for i, _ := range handles {
				_, actualHandle := volumizer.DestroyArgsForCall(i)
				actualHandles = append(actualHandles, actualHandle)
			}
			Expect(actualHandles).To(ConsistOf(handles))

			actualHandles = []string{}
			Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(len(handles)))
			for i, _ := range handles {
				actualHandle := propertyManager.DestroyKeySpaceArgsForCall(i)
				actualHandles = append(actualHandles, actualHandle)
			}
			Expect(actualHandles).To(ConsistOf(handles))

			actualHandles = []string{}
			Expect(containerizer.RemoveBundleCallCount()).To(Equal(len(handles)))
			for i, _ := range handles {
				_, actualHandle := containerizer.RemoveBundleArgsForCall(i)
				actualHandles = append(actualHandles, actualHandle)
			}
			Expect(actualHandles).To(ConsistOf(handles))
			Expect(logger.LogMessages()).ToNot(ContainElement(ContainSubstring("failed to remove container")))
		})

		Context("when pea cleanup fails", func() {
			BeforeEach(func() {
				peaCleaner.CleanAllReturns(errors.New("pea-cleanup-failure"))
			})

			It("returns the error", func() {
				Expect(cleanupErr).To(MatchError("clean peas: pea-cleanup-failure"))
			})

			It("exits", func() {
				Expect(restorer.RestoreCallCount()).To(Equal(0))
				Expect(containerizer.DestroyCallCount()).To(Equal(0))
				Expect(networker.DestroyCallCount()).To(Equal(0))
				Expect(volumizer.DestroyCallCount()).To(Equal(0))
				Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(0))
				Expect(containerizer.RemoveBundleCallCount()).To(Equal(0))
			})
		})

		Context("when retrieving the list of handles fails", func() {
			BeforeEach(func() {
				containerizer.HandlesReturns(nil, errors.New("handles-failure"))
			})

			It("returns the error", func() {
				Expect(cleanupErr).To(MatchError("handles-failure"))
			})

			It("exits", func() {
				Expect(restorer.RestoreCallCount()).To(Equal(0))
				Expect(containerizer.DestroyCallCount()).To(Equal(0))
				Expect(networker.DestroyCallCount()).To(Equal(0))
				Expect(volumizer.DestroyCallCount()).To(Equal(0))
				Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(0))
				Expect(containerizer.RemoveBundleCallCount()).To(Equal(0))
			})
		})

		Context("retrying on failure", func() {
			When("destroying one of the containers fails consistently", func() {
				BeforeEach(func() {
					networker.DestroyReturns(errors.New("networker-failure"))
				})

				It("retries for a total of cleanup retry limit attempts", func() {
					Expect(networker.DestroyCallCount()).To(Equal(2 * gardener.CleanupRetryLimit))
					Expect(logger.LogMessages()).To(ContainElement(ContainSubstring("failed to cleanup container")))
				})

				It("waits for cleanup retry sleep seconds before retrying", func() {
					Expect(sleeper.CallCount()).ToNot(BeZero())
					Expect(sleeper.ArgsForCall(0)).To(Equal(gardener.CleanupRetrySleep))
				})
			})

			When("destroying network fails and then starts working", func() {
				BeforeEach(func() {
					networker.DestroyReturns(nil)
					networker.DestroyReturnsOnCall(0, errors.New("networker-failure"))
				})

				It("stops retrying after a success", func() {
					Expect(sleeper.CallCount()).To(Equal(1))
					Expect(networker.DestroyCallCount()).To(Equal(3))
				})
			})
		})

		Context("when destroying one of the unrestorable containers consistently fails", func() {
			BeforeEach(func() {
				containerizer.DestroyReturns(errors.New("containerizer-failure"))
			})

			It("logs the failure and carries on", func() {
				Expect(cleanupErr).NotTo(HaveOccurred())
				Expect(containerizer.DestroyCallCount()).To(Equal(2 * gardener.CleanupRetryLimit))
				Expect(logger.Errors[0]).To(MatchError(ContainSubstring("containerizer-failure")))
			})
		})

		Context("when destroying one of the networks consistently fails", func() {
			BeforeEach(func() {
				networker.DestroyReturns(errors.New("networker-failure"))
			})

			It("logs the failure and carries on", func() {
				Expect(cleanupErr).NotTo(HaveOccurred())
				Expect(networker.DestroyCallCount()).To(Equal(2 * gardener.CleanupRetryLimit))
				Expect(logger.Errors[0]).To(MatchError(ContainSubstring("networker-failure")))
			})
		})

		Context("when destroying one of the volumes consistently fails", func() {
			BeforeEach(func() {
				volumizer.DestroyReturns(errors.New("volumizer-failure"))
			})

			It("logs the failure and carries on", func() {
				Expect(cleanupErr).NotTo(HaveOccurred())
				Expect(volumizer.DestroyCallCount()).To(Equal(2 * gardener.CleanupRetryLimit))
				Expect(logger.Errors[0]).To(MatchError(ContainSubstring("volumizer-failure")))
			})
		})

		Context("when destroying one of the property key spaces consistently fails", func() {
			BeforeEach(func() {
				propertyManager.DestroyKeySpaceReturns(errors.New("property-manager-failure"))
			})

			It("logs the failure and carries on", func() {
				Expect(cleanupErr).NotTo(HaveOccurred())
				Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(2 * gardener.CleanupRetryLimit))
				Expect(logger.Errors[0]).To(MatchError(ContainSubstring("property-manager-failure")))
			})
		})

		Context("when removing one of the bundles consistently fails", func() {
			BeforeEach(func() {
				containerizer.RemoveBundleReturns(errors.New("depot-failure"))
			})

			It("logs the failure and carries on", func() {
				Expect(cleanupErr).NotTo(HaveOccurred())
				Expect(containerizer.RemoveBundleCallCount()).To(Equal(2 * gardener.CleanupRetryLimit))
				Expect(logger.Errors[0]).To(MatchError("depot-failure"))
			})
		})
	})

	Describe("getting capacity", func() {
		BeforeEach(func() {
			sysinfoProvider.TotalMemoryReturns(999, nil)
			sysinfoProvider.TotalDiskReturns(888, nil)
			networker.CapacityReturns(1000)
			volumizer.CapacityReturns(777, nil)
		})

		It("returns capacity", func() {
			capacity, err := gdnr.Capacity()
			Expect(err).NotTo(HaveOccurred())

			Expect(capacity.MemoryInBytes).To(BeEquivalentTo(999))
			Expect(capacity.DiskInBytes).To(BeEquivalentTo(888))
			Expect(capacity.SchedulableDiskInBytes).To(BeEquivalentTo(777))
			Expect(capacity.MaxContainers).To(BeEquivalentTo(1000))
		})

		Context("when getting the total disk size fails", func() {
			BeforeEach(func() {
				sysinfoProvider.TotalDiskReturns(0, errors.New("sysinfo-provider-error"))
			})

			It("returns an error", func() {
				_, err := gdnr.Capacity()

				Expect(err).To(MatchError(errors.New("sysinfo-provider-error")))
			})
		})

		Context("getting the schedulable disk capacity", func() {
			BeforeEach(func() {
				volumizer.CapacityReturns(0, errors.New("capacity-error"))
			})

			It("returns the total disk size as the schedulable disk capacity", func() {
				capacity, err := gdnr.Capacity()
				Expect(err).NotTo(HaveOccurred())

				Expect(capacity.SchedulableDiskInBytes).To(BeEquivalentTo(888))
			})
		})

		Context("when MaxContainers is not set ", func() {
			BeforeEach(func() {
				gdnr.MaxContainers = 0
			})

			It("uses the network capacity", func() {
				capacity, err := gdnr.Capacity()
				Expect(err).NotTo(HaveOccurred())

				Expect(capacity.MaxContainers).To(BeEquivalentTo(1000))
			})
		})

		Context("when MaxContainers is set to less than the network capacity", func() {
			BeforeEach(func() {
				gdnr.MaxContainers = 1
			})

			It("uses MaxContainers", func() {
				capacity, err := gdnr.Capacity()
				Expect(err).NotTo(HaveOccurred())

				Expect(capacity.MaxContainers).To(BeEquivalentTo(1))
			})
		})

		Context("when MaxContainers is set to more than the network capacity", func() {
			BeforeEach(func() {
				gdnr.MaxContainers = 1001
			})

			It("uses the network capacity", func() {
				capacity, err := gdnr.Capacity()
				Expect(err).NotTo(HaveOccurred())

				Expect(capacity.MaxContainers).To(BeEquivalentTo(1000))
			})
		})

		Context("when getting the total memory fails", func() {
			BeforeEach(func() {
				sysinfoProvider.TotalMemoryReturns(0, errors.New("whelp"))
			})

			It("returns the error", func() {
				_, err := gdnr.Capacity()
				Expect(sysinfoProvider.TotalMemoryCallCount()).To(Equal(1))
				Expect(err).To(MatchError(errors.New("whelp")))
			})
		})
	})

	Describe("Properties", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("some-handle")
			Expect(err).NotTo(HaveOccurred())
		})

		It("delegates to the property manager for Properties", func() {
			container.Properties()
			Expect(propertyManager.AllCallCount()).To(Equal(1))
			handle := propertyManager.AllArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		It("delegates to the property manager for SetProperty", func() {
			container.SetProperty("name", "value")
			Expect(propertyManager.SetCallCount()).To(Equal(1))
			handle, prop, val := propertyManager.SetArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(prop).To(Equal("name"))
			Expect(val).To(Equal("value"))
		})

		It("delegates to the property manager for Property", func() {
			container.Property("name")
			Expect(propertyManager.GetCallCount()).To(Equal(1))
			handle, name := propertyManager.GetArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(name).To(Equal("name"))
		})

		It("delegates to the property manager for RemoveProperty", func() {
			container.RemoveProperty("name")
			Expect(propertyManager.RemoveCallCount()).To(Equal(1))
			handle, name := propertyManager.RemoveArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(name).To(Equal("name"))
		})
	})

	Describe("Info", func() {
		var container garden.Container

		var properties map[string]string

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("some-handle")
			Expect(err).NotTo(HaveOccurred())

			properties = make(map[string]string)
			properties[gardener.ContainerIPKey] = "1.2.3.4"
			properties[gardener.BridgeIPKey] = "8.9.10.11"
			properties[gardener.ExternalIPKey] = "4.5.6.7"

			propertyManager.GetStub = func(handle, key string) (string, bool) {
				Expect(handle).To(Equal("some-handle"))
				v, ok := properties[key]
				return v, ok
			}
		})

		It("returns state as 'active' or 'stopped' depending on whether the actual container is stopped", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.State).To(Equal("active"))

			containerizer.InfoReturns(spec.ActualContainerSpec{
				Stopped: true,
			}, nil)

			info, err = container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.State).To(Equal("stopped"))
		})

		It("returns the garden.network.container-ip property from the propertyManager as the ContainerIP", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.ContainerIP).To(Equal("1.2.3.4"))
		})

		It("returns the garden.network.host-ip property from the propertyManager as the HostIP", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.HostIP).To(Equal("8.9.10.11"))
		})

		It("returns the garden.network.external-ip property from the propertyManager as the ExternalIP", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.ExternalIP).To(Equal("4.5.6.7"))
		})

		It("returns the container path based on the info returned by the containerizer", func() {
			containerizer.InfoReturns(spec.ActualContainerSpec{
				BundlePath: "/foo/bar/baz",
			}, nil)

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.ContainerPath).To(Equal("/foo/bar/baz"))
		})

		Context("when getting the ActualContainerSpec fails", func() {
			It("return the error", func() {
				containerizer.InfoReturns(spec.ActualContainerSpec{}, errors.New("info-error"))

				_, err := container.Info()
				Expect(err).To(MatchError("info-error"))
			})
		})

		It("returns the container properties", func() {
			propertyManager.AllReturns(garden.Properties{
				"spider": "man",
				"super":  "man",
			}, nil)

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.Properties).To(Equal(garden.Properties{
				"spider": "man",
				"super":  "man",
			}))
		})

		Context("when the propertymanager fails to get properties", func() {
			It("should return the error", func() {
				propertyManager.AllReturns(garden.Properties{}, errors.New("hey-error"))

				_, err := container.Info()
				Expect(err).To(MatchError(("hey-error")))
			})
		})

		It("returns the list of mapped ports", func() {
			propertyManager.GetReturns(`[
			  {"HostPort":123,"ContainerPort":456},
			  {"HostPort":789,"ContainerPort":321}
			]`, true)
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.MappedPorts).To(HaveLen(2))

			portMapping1 := info.MappedPorts[0]
			Expect(portMapping1.HostPort).To(BeNumerically("==", 123))
			Expect(portMapping1.ContainerPort).To(BeNumerically("==", 456))

			portMapping2 := info.MappedPorts[1]
			Expect(portMapping2.HostPort).To(BeNumerically("==", 789))
			Expect(portMapping2.ContainerPort).To(BeNumerically("==", 321))
		})

		Context("when PropertyManager fails to get port mappings", func() {
			It("should return empty port mapping list", func() {
				delete(properties, gardener.MappedPortsKey)

				info, err := container.Info()
				Expect(err).NotTo(HaveOccurred())

				Expect(info.MappedPorts).To(BeEmpty())
			})
		})

		It("returns the events reported by the containerizer", func() {
			containerizer.InfoReturns(spec.ActualContainerSpec{
				Events: []string{"some", "things", "happened"},
			}, nil)

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Events).To(Equal([]string{
				"some", "things", "happened",
			}))
		})
	})

	Describe("BulkInfo", func() {
		var (
			container1 garden.Container
			container2 garden.Container
		)

		BeforeEach(func() {
			var err error
			container1, err = gdnr.Lookup("some-handle-1")
			Expect(err).NotTo(HaveOccurred())

			container2, err = gdnr.Lookup("some-handle-2")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the list of ContainerInfos of the containers", func() {
			infos, err := gdnr.BulkInfo([]string{"some-handle-1", "some-handle-2"})
			Expect(err).NotTo(HaveOccurred())

			Expect(infos).To(HaveKey("some-handle-1"))
			Expect(infos).To(HaveKey("some-handle-2"))

			info1, err := container1.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(infos["some-handle-1"].Info).To(Equal(info1))
			Expect(infos["some-handle-1"].Err).NotTo(HaveOccurred())

			info2, err := container2.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(infos["some-handle-2"].Info).To(Equal(info2))
			Expect(infos["some-handle-2"].Err).NotTo(HaveOccurred())
		})

		Context("when info errors", func() {
			It("returns the error", func() {
				containerizer.InfoReturns(spec.ActualContainerSpec{}, errors.New("info-error"))

				infos, err := gdnr.BulkInfo([]string{"some-handle-1"})
				Expect(err).NotTo(HaveOccurred())

				Expect(infos["some-handle-1"].Err).To(MatchError(ContainSubstring("info-error")))
			})
		})
	})

	Describe("Metrics", func() {
		var (
			container garden.Container

			cpuStat     garden.ContainerCPUStat
			memoryStat  garden.ContainerMemoryStat
			diskStat    garden.ContainerDiskStat
			networkStat *garden.ContainerNetworkStat
		)

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("some-handle")
			Expect(err).NotTo(HaveOccurred())

			cpuStat = garden.ContainerCPUStat{
				Usage:  12,
				System: 10,
				User:   11,
			}

			memoryStat = garden.ContainerMemoryStat{
				Cache: 10,
				Rss:   12,
			}

			diskStat = garden.ContainerDiskStat{
				TotalBytesUsed:      13,
				TotalInodesUsed:     14,
				ExclusiveBytesUsed:  15,
				ExclusiveInodesUsed: 16,
			}

			networkStat = &garden.ContainerNetworkStat{
				RxBytes: 42,
				TxBytes: 43,
			}

			containerizer.MetricsReturns(gardener.ActualContainerMetrics{
				StatsContainerMetrics: gardener.StatsContainerMetrics{
					CPU:    cpuStat,
					Memory: memoryStat,
					Age:    time.Minute,
				},
				CPUEntitlement: 12345,
			}, nil)

			volumizer.MetricsReturns(diskStat, nil)

			networkMetricsProvider.GetReturns(networkStat, nil)
		})

		It("should return the cpu and memory metrics from the containerizer", func() {
			metrics, err := container.Metrics()
			Expect(err).NotTo(HaveOccurred())

			Expect(metrics.CPUStat).To(Equal(cpuStat))
			Expect(metrics.MemoryStat).To(Equal(memoryStat))
		})

		It("should return the disk metrics from the volumizer", func() {
			metrics, err := container.Metrics()
			Expect(err).NotTo(HaveOccurred())

			Expect(metrics.DiskStat).To(Equal(diskStat))
		})

		It("should request disk metrics, informing that the volume is namespaced", func() {
			_, err := container.Metrics()
			Expect(err).NotTo(HaveOccurred())

			Expect(volumizer.MetricsCallCount()).To(Equal(1))
			_, _, namespaced := volumizer.MetricsArgsForCall(0)
			Expect(namespaced).To(BeTrue())
		})

		It("should return the container age", func() {
			metrics, err := container.Metrics()
			Expect(err).NotTo(HaveOccurred())

			Expect(metrics.Age).To(Equal(time.Minute))
		})

		It("should return the container CPU entitlement", func() {
			metrics, err := container.Metrics()
			Expect(err).NotTo(HaveOccurred())

			Expect(metrics.CPUEntitlement).To(Equal(uint64(12345)))
		})

		It("should return the network statistics", func() {
			metrics, err := container.Metrics()
			Expect(err).NotTo(HaveOccurred())

			Expect(metrics.NetworkStat.TxBytes).To(Equal(networkStat.TxBytes))
			Expect(metrics.NetworkStat.RxBytes).To(Equal(networkStat.RxBytes))
		})

		Context("when the container is privileged", func() {
			BeforeEach(func() {
				volumizer.MetricsStub = func(_ lager.Logger, _ string, namespaced bool) (garden.ContainerDiskStat, error) {
					if namespaced {
						return garden.ContainerDiskStat{}, errors.New("not found")
					}

					return garden.ContainerDiskStat{}, nil
				}
			})

			It("returns the disk metrics from the volumizer after trying fetching it as namespaced first", func() {
				_, err := container.Metrics()
				Expect(err).NotTo(HaveOccurred())

				Expect(volumizer.MetricsCallCount()).To(Equal(2))
				_, _, namespaced := volumizer.MetricsArgsForCall(1)
				Expect(namespaced).To(BeFalse())
			})
		})

		Context("when cpu/mem metrics cannot be acquired", func() {
			BeforeEach(func() {
				containerizer.MetricsReturns(gardener.ActualContainerMetrics{}, errors.New("banana"))
			})

			It("should propagate the error", func() {
				_, err := container.Metrics()
				Expect(err).To(MatchError("banana"))
			})
		})

		Context("when disk metrics cannot be acquired", func() {
			BeforeEach(func() {
				volumizer.MetricsReturns(garden.ContainerDiskStat{}, errors.New("banana"))
			})

			It("should propagate the error", func() {
				_, err := container.Metrics()
				Expect(err).To(MatchError(ContainSubstring("banana")))
			})
		})

		Context("when the network metrics cannot be acquired", func() {
			BeforeEach(func() {
				networkMetricsProvider.GetReturns(nil, errors.New("processError"))
			})

			It("should propagate the error", func() {
				_, err := container.Metrics()
				Expect(err).To(MatchError(ContainSubstring("processError")))
			})
		})

		Context("when the network metrics are missing", func() {
			BeforeEach(func() {
				networkMetricsProvider.GetReturns(nil, nil)
			})

			It("should not return an error", func() {
				_, err := container.Metrics()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		It("should return BulkMetrics", func() {
			containerizer.MetricsStub = func(_ lager.Logger, id string) (gardener.ActualContainerMetrics, error) {
				if id == "potato" {
					return gardener.ActualContainerMetrics{}, errors.New("potatoError")
				}

				return gardener.ActualContainerMetrics{
					StatsContainerMetrics: gardener.StatsContainerMetrics{
						CPU:    cpuStat,
						Memory: memoryStat,
					},
				}, nil
			}

			metrics, err := gdnr.BulkMetrics([]string{"some-handle", "potato"})
			Expect(err).NotTo(HaveOccurred())

			Expect(metrics).To(HaveKeyWithValue("some-handle", garden.ContainerMetricsEntry{
				Metrics: garden.Metrics{
					DiskStat:    diskStat,
					MemoryStat:  memoryStat,
					CPUStat:     cpuStat,
					NetworkStat: networkStat,
				},
			}))

			Expect(metrics).To(HaveKeyWithValue("potato", garden.ContainerMetricsEntry{
				Err: garden.NewError("potatoError"),
			}))
		})
	})

	Describe("Limits", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("some-handle")
			Expect(err).NotTo(HaveOccurred())
		})

		It("gets the set CPU limits", func() {
			containerizer.InfoReturns(spec.ActualContainerSpec{
				Limits: garden.Limits{
					CPU: garden.CPULimits{
						LimitInShares: 10,
					},
				},
			}, nil)

			currentCPULimits, err := container.CurrentCPULimits()
			Expect(err).ToNot(HaveOccurred())
			Expect(currentCPULimits.LimitInShares).To(BeEquivalentTo(10))
		})

		It("gets the set memory limits", func() {
			containerizer.InfoReturns(spec.ActualContainerSpec{
				Limits: garden.Limits{
					Memory: garden.MemoryLimits{
						LimitInBytes: 20,
					},
				},
			}, nil)

			currentMemoryLimits, err := container.CurrentMemoryLimits()
			Expect(err).ToNot(HaveOccurred())
			Expect(currentMemoryLimits.LimitInBytes).To(BeEquivalentTo(20))
		})

		Context("when Info fails", func() {
			It("forwards the error", func() {
				containerizer.InfoReturns(spec.ActualContainerSpec{}, errors.New("some-error"))

				_, err := container.CurrentCPULimits()
				Expect(err).To(MatchError("some-error"))

				_, err = container.CurrentMemoryLimits()
				Expect(err).To(MatchError("some-error"))
			})
		})
	})

	Describe("GraceTime", func() {
		var container garden.Container

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("some-handle")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the grace time property is present", func() {
			var graceTime time.Duration

			BeforeEach(func() {
				graceTime = time.Minute

				propertyManager.GetReturns(fmt.Sprintf("%d", graceTime), true)
			})

			It("returns the parsed duration", func() {
				Expect(gdnr.GraceTime(container)).To(Equal(time.Minute))

				Expect(propertyManager.GetCallCount()).To(Equal(1))
				handle, name := propertyManager.GetArgsForCall(0)
				Expect(handle).To(Equal("some-handle"))
				Expect(name).To(Equal(gardener.GraceTimeKey))
			})
		})

		Context("when getting the grace time fails (i.e. property not found)", func() {
			BeforeEach(func() {
				propertyManager.GetReturns("", false)
			})

			It("returns no grace time", func() {
				Expect(gdnr.GraceTime(container)).To(BeZero())
			})
		})
	})

	Describe("Stop", func() {
		var stopErr error

		JustBeforeEach(func() {
			stopErr = gdnr.Stop()
		})

		It("shuts down the containerizer", func() {
			Expect(stopErr).NotTo(HaveOccurred())
			Expect(containerizer.ShutdownCallCount()).To(Equal(1))
		})

		Context("when the containerizer fails to shutdown", func() {
			BeforeEach(func() {
				containerizer.ShutdownReturns(errors.New("shutdown-err"))
			})

			It("returns the error", func() {
				Expect(stopErr).To(MatchError("shutdown-err"))
			})
		})
	})
})

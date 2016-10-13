package gardener_test

import (
	"errors"
	"fmt"
	"net"
	"time"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/gardener"
	fakes "code.cloudfoundry.org/guardian/gardener/gardenerfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Gardener", func() {
	var (
		networker       *fakes.FakeNetworker
		volumeCreator   *fakes.FakeVolumeCreator
		containerizer   *fakes.FakeContainerizer
		uidGenerator    *fakes.FakeUidGenerator
		sysinfoProvider *fakes.FakeSysInfoProvider
		propertyManager *fakes.FakePropertyManager
		restorer        *fakes.FakeRestorer

		logger lager.Logger

		gdnr *gardener.Gardener
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		containerizer = new(fakes.FakeContainerizer)
		uidGenerator = new(fakes.FakeUidGenerator)
		networker = new(fakes.FakeNetworker)
		volumeCreator = new(fakes.FakeVolumeCreator)
		sysinfoProvider = new(fakes.FakeSysInfoProvider)
		propertyManager = new(fakes.FakePropertyManager)
		restorer = new(fakes.FakeRestorer)

		propertyManager.GetReturns("", true)
		containerizer.HandlesReturns([]string{"some-handle"}, nil)

		gdnr = &gardener.Gardener{
			SysInfoProvider: sysinfoProvider,
			Containerizer:   containerizer,
			UidGenerator:    uidGenerator,
			Networker:       networker,
			VolumeCreator:   volumeCreator,
			Logger:          logger,
			PropertyManager: propertyManager,
			Restorer:        restorer,
		}
	})

	Describe("creating a container", func() {
		ItDestroysEverything := func(rootfsPath string) {
			BeforeEach(func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					RootFSPath: rootfsPath,
					Handle:     "poor-banana",
				})
				Expect(err).To(HaveOccurred())
			})

			It("should clean up the networking configuration", func() {
				Expect(networker.DestroyCallCount()).To(Equal(1))
				_, handle := networker.DestroyArgsForCall(0)
				Expect(handle).To(Equal("poor-banana"))
			})

			It("should clean up any created volumes", func() {
				Expect(volumeCreator.DestroyCallCount()).To(Equal(1))
				_, handle := volumeCreator.DestroyArgsForCall(0)
				Expect(handle).To(Equal("poor-banana"))
			})

			It("should destroy any container state", func() {
				Expect(containerizer.DestroyCallCount()).To(Equal(1))
				_, handle := containerizer.DestroyArgsForCall(0)
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

			Expect(volumeCreator.GCCallCount()).To(Equal(1))
		})

		Context("when the graph cleanup fails", func() {
			It("does NOT return the error", func() {
				volumeCreator.GCReturns(errors.New("graph-cleanup-fail"))
				_, err := gdnr.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when parsing the rootfs path fails", func() {
			It("should return an error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					RootFSPath: "://banana",
				})
				Expect(err).To(HaveOccurred())
			})

			ItDestroysEverything("://banana")
		})

		Context("when the rootfs path is raw", func() {
			BeforeEach(func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					Handle:     "bob",
					RootFSPath: "raw:///banana",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates the container with the given path", func() {
				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.RootFSPath).To(Equal("/banana"))
			})

			It("does not create a volume", func() {
				Expect(volumeCreator.CreateCallCount()).To(Equal(0))
			})
		})

		It("should ask the shed for a namespaced rootfs", func() {
			_, err := gdnr.Create(garden.ContainerSpec{})
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeCreator.CreateCallCount()).To(Equal(1))
			_, _, fsSpec := volumeCreator.CreateArgsForCall(0)
			Expect(fsSpec.Namespaced).To(BeTrue())
		})

		Context("when the container is privileged", func() {
			It("should ask the shed for an unnamespaced rootfs", func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					Privileged: true,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(volumeCreator.CreateCallCount()).To(Equal(1))
				_, _, fsSpec := volumeCreator.CreateArgsForCall(0)
				Expect(fsSpec.Namespaced).To(BeFalse())
			})
		})

		It("passes the created rootfs to the containerizer", func() {
			volumeCreator.CreateStub = func(_ lager.Logger, handle string, spec rootfs_provider.Spec) (string, []string, error) {
				return "/path/to/rootfs/" + spec.RootFS.String() + "/" + handle, []string{}, nil
			}

			_, err := gdnr.Create(garden.ContainerSpec{
				Handle:     "bob",
				RootFSPath: "alice",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.RootFSPath).To(Equal("/path/to/rootfs/alice/bob"))
		})

		Context("when volume creator fails", func() {
			BeforeEach(func() {
				volumeCreator.CreateReturns("", []string{}, errors.New("booom!"))
			})

			It("returns an error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(MatchError("booom!"))
			})

			It("should not call the containerizer", func() {
				gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(containerizer.CreateCallCount()).To(Equal(0))
			})

			ItDestroysEverything("")
		})

		It("asks the containerizer to create a container", func() {
			_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob", Privileged: true})
			Expect(err).NotTo(HaveOccurred())

			Expect(containerizer.CreateCallCount()).To(Equal(1))
			_, spec := containerizer.CreateArgsForCall(0)
			Expect(spec.Handle).To(Equal("bob"))
			Expect(spec.Privileged).To(BeTrue())
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

			ItDestroysEverything("")

			It("logs the underlying error", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(HaveOccurred())
				Eventually(logger).Should(gbytes.Say("failed to create the banana"))
			})
		})

		Context("when a disk limit is provided", func() {
			var spec garden.ContainerSpec

			BeforeEach(func() {
				spec.Limits.Disk.Scope = garden.DiskLimitScopeTotal
				spec.Limits.Disk.ByteHard = 10 * 1024 * 1024
			})

			It("should delegate the limit to the volume creator", func() {
				_, err := gdnr.Create(spec)
				Expect(err).NotTo(HaveOccurred())

				Expect(volumeCreator.CreateCallCount()).To(Equal(1))
				_, _, rpSpec := volumeCreator.CreateArgsForCall(0)
				Expect(rpSpec.QuotaSize).To(BeEquivalentTo(spec.Limits.Disk.ByteHard))
				Expect(rpSpec.QuotaScope).To(Equal(garden.DiskLimitScopeTotal))
			})
		})

		It("should ask the networker to configure the network", func() {
			containerizer.InfoReturns(gardener.ActualContainerSpec{
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
				containerizer.InfoReturns(gardener.ActualContainerSpec{}, errors.New("boom"))

				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(MatchError("boom"))

			})
		})

		Context("when networker fails to configure network", func() {
			It("errors", func() {
				networker.NetworkReturns(errors.New("network-failed"))

				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
				Expect(err).To(MatchError("network-failed"))
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

		Context("when a memory limit is provided", func() {
			It("should pass the memory limit to the containerizer", func() {
				memLimit := garden.Limits{
					Memory: garden.MemoryLimits{LimitInBytes: 4096},
				}

				_, err := gdnr.Create(garden.ContainerSpec{
					Limits: memLimit,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))

				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.Limits).To(Equal(memLimit))
			})
		})

		Context("when environment variables are returned by the volume manager", func() {
			It("passes them to the containerizer", func() {
				volumeCreator.CreateStub = func(_ lager.Logger, handle string, spec rootfs_provider.Spec) (string, []string, error) {
					return "", []string{"foo=bar", "name=blame"}, nil
				}

				_, err := gdnr.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.Env).To(Equal([]string{"foo=bar", "name=blame"}))
			})
		})

		Context("when environment variables are specified", func() {
			It("passes into the containerizer", func() {
				_, err := gdnr.Create(garden.ContainerSpec{
					Env: []string{"ENV.CONTAINER_ID=1", "ENV.CONTAINER_NAME=garden"},
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.Env).To(Equal([]string{
					"ENV.CONTAINER_ID=1",
					"ENV.CONTAINER_NAME=garden",
				}))
			})
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

			Context("when containerizer.Handles() returns an error", func() {
				BeforeEach(func() {
					containerizer.HandlesReturns(nil, errors.New("boom"))
				})

				It("forwards the error", func() {
					containerSpec := garden.ContainerSpec{Handle: "duplicate-banana"}
					container, err := gdnr.Create(containerSpec)
					Expect(container).To(BeNil())
					Expect(err).To(MatchError("boom"))
				})
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
	})

	Describe("starting up gardener", func() {
		BeforeEach(func() {
			containers := []string{"container1", "container2"}
			containerizer.HandlesReturns(containers, nil)
		})

		Context("when it has starters", func() {
			var (
				starterA, starterB *fakes.FakeStarter
			)

			BeforeEach(func() {
				starterA = new(fakes.FakeStarter)
				starterB = new(fakes.FakeStarter)
				gdnr.Starters = []gardener.Starter{starterA, starterB}
			})

			It("calls the provided starters", func() {
				Expect(gdnr.Start()).To(Succeed())

				Expect(starterA.StartCallCount()).To(Equal(1))
				Expect(starterB.StartCallCount()).To(Equal(1))
			})

			Context("when a starter fails", func() {
				BeforeEach(func() {
					starterB.StartReturns(errors.New("boom"))
				})

				It("returns the error", func() {
					Expect(gdnr.Start()).To(MatchError(ContainSubstring("boom")))
				})

				It("does not restore the containers", func() {
					Expect(gdnr.Start()).NotTo(Succeed())
					Expect(restorer.RestoreCallCount()).To(Equal(0))
				})
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
			Expect(gdnr.Start()).To(MatchError("banana"))
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
	})

	Describe("NetIn", func() {
		var container garden.Container

		const (
			externalPort  uint32 = 8888
			contianerPort uint32 = 8080
		)

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("banana")
			Expect(err).NotTo(HaveOccurred())
		})

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
		var (
			container garden.Container
			rule      garden.NetOutRule
		)

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("banana")
			Expect(err).NotTo(HaveOccurred())

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
		It("returns garden.ContainreNotFoundError if the container handle isn't in the depot", func() {
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
			networkLogger, handleToDestroy := networker.DestroyArgsForCall(0)
			Expect(handleToDestroy).To(Equal("some-handle"))
			Expect(networkLogger).To(Equal(logger))
		})

		It("asks the volume creator to destroy the container rootfs", func() {
			gdnr.Destroy("some-handle")
			Expect(volumeCreator.DestroyCallCount()).To(Equal(1))
			_, handleToDestroy := volumeCreator.DestroyArgsForCall(0)
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
				Expect(err).To(MatchError("containerized deletion failed"))
			})

			It("should not destroy the network configuration", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(networker.DestroyCallCount()).To(Equal(0))
			})
		})

		Context("when containerizer fails to remove the bundle from the depot", func() {
			BeforeEach(func() {
				containerizer.DestroyReturns(errors.New("containerized deletion failed"))
			})

			It("return the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError("containerized deletion failed"))
			})
		})

		Context("when network deletion fails", func() {
			BeforeEach(func() {
				networker.DestroyReturns(errors.New("network deletion failed"))
			})

			It("returns the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError("network deletion failed"))
			})

			It("should not destroy the key space of the property manager", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(HaveOccurred())

				Expect(propertyManager.DestroyKeySpaceCallCount()).To(Equal(0))
			})
		})

		Context("when destroying the rootfs fails", func() {
			BeforeEach(func() {
				volumeCreator.DestroyReturns(errors.New("rootfs deletion failed"))
			})

			It("returns the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError("rootfs deletion failed"))
			})
		})

		Context("when destroying key space fails", func() {
			BeforeEach(func() {
				propertyManager.DestroyKeySpaceReturns(errors.New("key space destruction failed"))
			})

			It("returns the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError("key space destruction failed"))
			})
		})

		Context("when containerizer fails to remove the bundle from the depot", func() {
			BeforeEach(func() {
				containerizer.RemoveBundleReturns(errors.New("bundle removal failed"))
			})

			It("return the error", func() {
				err := gdnr.Destroy("some-handle")
				Expect(err).To(MatchError("bundle removal failed"))
			})
		})
	})

	Describe("getting capacity", func() {
		BeforeEach(func() {
			sysinfoProvider.TotalMemoryReturns(999, nil)
			sysinfoProvider.TotalDiskReturns(888, nil)
			networker.CapacityReturns(1000)
		})

		It("returns capacity", func() {
			capacity, err := gdnr.Capacity()
			Expect(err).NotTo(HaveOccurred())

			Expect(capacity.MemoryInBytes).To(BeEquivalentTo(999))
			Expect(capacity.DiskInBytes).To(BeEquivalentTo(888))
			Expect(capacity.MaxContainers).To(BeEquivalentTo(1000))
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

		Context("when getting the total disk fails", func() {
			BeforeEach(func() {
				sysinfoProvider.TotalDiskReturns(0, errors.New("whelp"))
			})

			It("returns the error", func() {
				_, err := gdnr.Capacity()
				Expect(sysinfoProvider.TotalDiskCallCount()).To(Equal(1))
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

			containerizer.InfoReturns(gardener.ActualContainerSpec{
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

		Context("when getting the containerIP fails", func() {
			It("should return the error", func() {
				delete(properties, gardener.ContainerIPKey)
				_, err := container.Info()
				Expect(err).To(MatchError(MatchRegexp("no property found.*container-ip")))
			})
		})

		It("returns the garden.network.host-ip property from the propertyManager as the HostIP", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.HostIP).To(Equal("8.9.10.11"))
		})

		Context("when getting the hostIP fails", func() {
			It("should return the error", func() {
				delete(properties, gardener.BridgeIPKey)
				_, err := container.Info()
				Expect(err).To(MatchError(MatchRegexp("no property found.*host-ip")))
			})
		})

		It("returns the garden.network.external-ip property from the propertyManager as the ExternalIP", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())
			Expect(info.ExternalIP).To(Equal("4.5.6.7"))
		})

		Context("when getting the externalIP fails", func() {
			It("should return the error", func() {
				delete(properties, gardener.ExternalIPKey)

				_, err := container.Info()
				Expect(err).To(MatchError(MatchRegexp("no property found.*external-ip")))
			})
		})

		It("returns the container path based on the info returned by the containerizer", func() {
			containerizer.InfoReturns(gardener.ActualContainerSpec{
				BundlePath: "/foo/bar/baz",
			}, nil)

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.ContainerPath).To(Equal("/foo/bar/baz"))
		})

		Context("when getting the ActualContainerSpec fails", func() {
			It("return the error", func() {
				containerizer.InfoReturns(gardener.ActualContainerSpec{}, errors.New("info-error"))

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
			containerizer.InfoReturns(gardener.ActualContainerSpec{
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
				propertyManager.GetReturns("", false)

				infos, err := gdnr.BulkInfo([]string{"some-handle-1"})
				Expect(err).NotTo(HaveOccurred())

				Expect(infos["some-handle-1"].Err).To(MatchError(ContainSubstring("no property found")))
			})
		})
	})

	Describe("Metrics", func() {
		var (
			container garden.Container

			cpuStat    garden.ContainerCPUStat
			memoryStat garden.ContainerMemoryStat
			diskStat   garden.ContainerDiskStat
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

			containerizer.MetricsReturns(gardener.ActualContainerMetrics{
				CPU:    cpuStat,
				Memory: memoryStat,
			}, nil)

			volumeCreator.MetricsReturns(diskStat, nil)
		})

		It("should return the cpu and memory metrics the containerizer", func() {
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

		Context("when cpu/mem metrics cannot be acquired", func() {
			BeforeEach(func() {
				containerizer.MetricsReturns(gardener.ActualContainerMetrics{}, errors.New("banana"))
			})

			It("should propagete the error", func() {
				_, err := container.Metrics()
				Expect(err).To(MatchError("banana"))
			})
		})

		Context("when disk metrics cannot be acquired", func() {
			BeforeEach(func() {
				volumeCreator.MetricsReturns(garden.ContainerDiskStat{}, errors.New("banana"))
			})

			It("should propagete the error", func() {
				_, err := container.Metrics()
				Expect(err).To(MatchError("banana"))
			})
		})

		It("should return BulkMetrics", func() {
			containerizer.MetricsStub = func(_ lager.Logger, id string) (gardener.ActualContainerMetrics, error) {
				if id == "potato" {
					return gardener.ActualContainerMetrics{}, errors.New("potatoError")
				}

				return gardener.ActualContainerMetrics{
					CPU:    cpuStat,
					Memory: memoryStat,
				}, nil
			}

			metrics, err := gdnr.BulkMetrics([]string{"some-handle", "potato"})
			Expect(err).NotTo(HaveOccurred())

			Expect(metrics).To(HaveKeyWithValue("some-handle", garden.ContainerMetricsEntry{
				Metrics: garden.Metrics{
					DiskStat:   diskStat,
					MemoryStat: memoryStat,
					CPUStat:    cpuStat,
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
			containerizer.InfoReturns(gardener.ActualContainerSpec{
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
			containerizer.InfoReturns(gardener.ActualContainerSpec{
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
				containerizer.InfoReturns(gardener.ActualContainerSpec{}, errors.New("some-error"))

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
})

package gardener_test

import (
	"errors"
	"fmt"
	"net"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden-shed/rootfs_provider"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/gardener/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Gardener", func() {
	var (
		networker       *fakes.FakeNetworker
		volumeCreator   *fakes.FakeVolumeCreator
		containerizer   *fakes.FakeContainerizer
		uidGenerator    *fakes.FakeUidGenerator
		sysinfoProvider *fakes.FakeSysInfoProvider
		propertyManager *fakes.FakePropertyManager

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

		gdnr = &gardener.Gardener{
			SysInfoProvider: sysinfoProvider,
			Containerizer:   containerizer,
			UidGenerator:    uidGenerator,
			Networker:       networker,
			VolumeCreator:   volumeCreator,
			Logger:          logger,
			PropertyManager: propertyManager,
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

		Context("when a handle is specified", func() {
			It("passes the network hooks to the containerizer", func() {
				networker.HooksStub = func(_ lager.Logger, handle, spec string) (gardener.Hooks, error) {
					return gardener.Hooks{
						Prestart: gardener.Hook{
							Path: "/path/to/banana/exe",
							Args: []string{"--handle", handle, "--spec", spec},
						},
						Poststop: gardener.Hook{
							Path: "/path/to/bananana/exe",
							Args: []string{"--handle", handle, "--spec", spec},
						},
					}, nil
				}

				_, err := gdnr.Create(garden.ContainerSpec{
					Handle:  "bob",
					Network: "10.0.0.2/30",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.NetworkHooks.Prestart).To(Equal(gardener.Hook{
					Path: "/path/to/banana/exe",
					Args: []string{"--handle", "bob", "--spec", "10.0.0.2/30"},
				}))

				Expect(spec.NetworkHooks.Poststop).To(Equal(gardener.Hook{
					Path: "/path/to/bananana/exe",
					Args: []string{"--handle", "bob", "--spec", "10.0.0.2/30"},
				}))
			})

			Context("when networker fails", func() {
				BeforeEach(func() {
					networker.HooksReturns(gardener.Hooks{}, errors.New("booom!"))
				})

				It("returns an error", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
					Expect(err).To(MatchError("booom!"))
				})

				It("should not create the volume", func() {
					gdnr.Create(garden.ContainerSpec{Handle: "bob"})
					Expect(volumeCreator.CreateCallCount()).To(Equal(0))
				})
			})

			It("run the graph cleanup", func() {
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

			Context("when parsing the rootfs path fails", func() {
				It("should return an error", func() {
					_, err := gdnr.Create(garden.ContainerSpec{
						RootFSPath: "://banana",
					})
					Expect(err).To(HaveOccurred())
				})

				ItDestroysEverything("://banana")
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

			Context("when the rootfs path is raw", func() {
				It("creates the container with the given path", func() {
					_, err := gdnr.Create(garden.ContainerSpec{
						Handle:     "bob",
						RootFSPath: "raw:///banana",
					})
					Expect(err).NotTo(HaveOccurred())

					Expect(containerizer.CreateCallCount()).To(Equal(1))
					_, spec := containerizer.CreateArgsForCall(0)
					Expect(spec.RootFSPath).To(Equal("/banana"))
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

			It("asks the containerizer to create a container", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob", Privileged: true})

				Expect(err).NotTo(HaveOccurred())
				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.Handle).To(Equal("bob"))
				Expect(spec.Privileged).To(BeTrue())
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
			})

			It("returns the container that Lookup would return", func() {
				c, err := gdnr.Create(garden.ContainerSpec{Handle: "handle"})
				Expect(err).NotTo(HaveOccurred())

				d, err := gdnr.Lookup("handle")
				Expect(err).NotTo(HaveOccurred())
				Expect(c).To(Equal(d))
			})
		})

		Context("when no handle is specified", func() {
			It("assigns a handle to the container", func() {
				uidGenerator.GenerateReturns("generated-handle")

				_, err := gdnr.Create(garden.ContainerSpec{})

				Expect(err).NotTo(HaveOccurred())
				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.Handle).To(Equal("generated-handle"))
			})

			It("returns the container that Lookup would return", func() {
				c, err := gdnr.Create(garden.ContainerSpec{})
				Expect(err).NotTo(HaveOccurred())

				d, err := gdnr.Lookup(c.Handle())
				Expect(err).NotTo(HaveOccurred())
				Expect(c).To(Equal(d))
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

				Expect(propertyManager.SetCallCount()).To(Equal(2))

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

		Context("when bind mounts are specified", func() {
			It("generates a proper mount spec", func() {
				bindMounts := []garden.BindMount{
					garden.BindMount{
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

	Describe("listing containers", func() {
		BeforeEach(func() {
			containerizer.HandlesReturns([]string{"banana", "banana2", "cola"}, nil)
		})

		It("should return matching containers", func() {
			propertyManager.MatchesAllStub = func(handle string, props garden.Properties) bool {
				if handle != "banana" {
					return true
				}
				return false
			}

			c, err := gdnr.Containers(garden.Properties{
				"somename": "somevalue",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(c).To(HaveLen(2))
			Expect(c[0].Handle()).To(Equal("banana2"))
			Expect(c[1].Handle()).To(Equal("cola"))
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

	Describe("destroying a container", func() {
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
		var propertyMgrErrors map[string]error

		BeforeEach(func() {
			var err error
			container, err = gdnr.Lookup("some-handle")
			Expect(err).NotTo(HaveOccurred())

			properties = make(map[string]string)
			propertyMgrErrors = make(map[string]error)
			propertyManager.GetStub = func(handle, key string) (string, error) {
				Expect(handle).To(Equal("some-handle"))
				return properties[key], propertyMgrErrors[key]
			}
		})

		It("hard-codes the state to 'active'", func() {
			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.State).To(Equal("active"))
		})

		It("returns the garden.network.container-ip property from the propertyManager as the ContainerIP", func() {
			properties[gardener.ContainerIPKey] = "1.2.3.4"

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.ContainerIP).To(Equal("1.2.3.4"))
		})

		Context("when getting the containerIP fails", func() {
			It("should return the error", func() {
				propertyMgrErrors[gardener.ContainerIPKey] = errors.New("spiderman-error")

				_, err := container.Info()
				Expect(err).To(MatchError("spiderman-error"))
			})
		})

		It("returns the garden.network.host-ip property from the propertyManager as the HostIP", func() {
			properties[gardener.BridgeIPKey] = "1.2.3.4"

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.HostIP).To(Equal("1.2.3.4"))
		})

		Context("when getting the hostIP fails", func() {
			It("should return the error", func() {
				propertyMgrErrors[gardener.BridgeIPKey] = errors.New("spiderman-error")

				_, err := container.Info()
				Expect(err).To(MatchError("spiderman-error"))
			})
		})

		It("returns the garden.network.external-ip property from the propertyManager as the ExternalIP", func() {
			properties[gardener.ExternalIPKey] = "1.2.3.4"

			info, err := container.Info()
			Expect(err).NotTo(HaveOccurred())

			Expect(info.ExternalIP).To(Equal("1.2.3.4"))
		})

		Context("when getting the externalIP fails", func() {
			It("should return the error", func() {
				propertyMgrErrors[gardener.ExternalIPKey] = errors.New("spiderman-error")

				_, err := container.Info()
				Expect(err).To(MatchError("spiderman-error"))
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
			]`, nil)
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
				propertyMgrErrors[gardener.MappedPortsKey] = errors.New("spiderman-error")

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
				propertyManager.GetReturns("", errors.New("boom"))

				infos, err := gdnr.BulkInfo([]string{"some-handle-1"})
				Expect(err).NotTo(HaveOccurred())

				Expect(infos["some-handle-1"].Err).To(MatchError("boom"))
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
})

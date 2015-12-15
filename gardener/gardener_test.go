package gardener_test

import (
	"errors"

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

		gdnr *gardener.Gardener
	)

	BeforeEach(func() {
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
			Logger:          lagertest.NewTestLogger("test"),
			PropertyManager: propertyManager,
		}
	})

	Describe("creating a container", func() {
		Context("when a handle is specified", func() {
			It("asks the containerizer to create a container", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})

				Expect(err).NotTo(HaveOccurred())
				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.Handle).To(Equal("bob"))
			})

			It("passes the created network to the containerizer", func() {
				networker.NetworkStub = func(_ lager.Logger, handle, spec string) (string, error) {
					return "/path/to/netns/" + handle, nil
				}

				_, err := gdnr.Create(garden.ContainerSpec{
					Handle:  "bob",
					Network: "10.0.0.2/30",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(containerizer.CreateCallCount()).To(Equal(1))
				_, spec := containerizer.CreateArgsForCall(0)
				Expect(spec.NetworkPath).To(Equal("/path/to/netns/bob"))
			})

			Context("when networker fails", func() {
				BeforeEach(func() {
					networker.NetworkReturns("", errors.New("booom!"))
				})

				It("returns an error", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
					Expect(err).To(MatchError("booom!"))
				})

				It("does not create a container", func() {
					gdnr.Create(garden.ContainerSpec{Handle: "bob"})
					Expect(containerizer.CreateCallCount()).To(Equal(0))
				})
			})

			It("passes the created rootfs to the containerizer", func() {
				volumeCreator.CreateStub = func(handle string, spec rootfs_provider.Spec) (string, []string, error) {
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

				It("does not create a container", func() {
					gdnr.Create(garden.ContainerSpec{Handle: "bob"})
					Expect(containerizer.CreateCallCount()).To(Equal(0))
				})
			})

			It("creates a key space for container properties", func() {
				_, err := gdnr.Create(garden.ContainerSpec{Handle: "some-handle"})

				Expect(err).NotTo(HaveOccurred())
				Expect(propertyManager.CreateKeySpaceCallCount()).To(Equal(1))
				Expect(propertyManager.CreateKeySpaceArgsForCall(0)).To(Equal("some-handle"))
			})

			Context("when creating the key space fails", func() {
				BeforeEach(func() {
					propertyManager.CreateKeySpaceReturns(errors.New("kabluey"))
				})

				It("returns the error", func() {
					_, err := gdnr.Create(garden.ContainerSpec{Handle: "bob"})
					Expect(err).To(MatchError("kabluey"))
				})
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

			Context("when error on set property occurs", func() {
				It("returns the error", func() {
					propertyManager.SetReturns(errors.New("error"))

					_, err := gdnr.Create(garden.ContainerSpec{
						Properties: startingProperties,
					})
					Expect(err).To(MatchError(errors.New("error")))
				})
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

		Describe("destroying a container", func() {
			It("asks the containerizer to destroy the container", func() {
				Expect(gdnr.Destroy(container.Handle())).To(Succeed())
				Expect(containerizer.DestroyCallCount()).To(Equal(1))
				_, handle := containerizer.DestroyArgsForCall(0)
				Expect(handle).To(Equal(container.Handle()))
			})

			It("removes the key space from property manager", func() {
				Expect(gdnr.Destroy(container.Handle())).To(Succeed())
				Expect(propertyManager.DestroyKeySpaceArgsForCall(0)).To(Equal(container.Handle()))
			})

			Context("when an error occurs deleting a key space", func() {
				It("returns the error", func() {
					propertyManager.DestroyKeySpaceReturns(errors.New("some error"))
					err := gdnr.Destroy(container.Handle())
					Expect(err).To(MatchError(errors.New("some error")))
				})
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
})

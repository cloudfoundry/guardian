package configure_test

import (
	"errors"
	"io/ioutil"
	"net"
	"os"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/configure"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/devices/fakedevices"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Host", func() {
	var (
		vethCreator    *fakedevices.FaveVethCreator
		linkConfigurer *fakedevices.FakeLink
		bridger        *fakedevices.FakeBridge

		configurer *configure.Host

		logger lager.Logger
		config kawasaki.NetworkConfig
	)

	BeforeEach(func() {
		vethCreator = &fakedevices.FaveVethCreator{}
		linkConfigurer = &fakedevices.FakeLink{AddIPReturns: make(map[string]error)}
		bridger = &fakedevices.FakeBridge{}

		logger = lagertest.NewTestLogger("test")
		configurer = &configure.Host{Veth: vethCreator, Link: linkConfigurer, Bridge: bridger}

		config = kawasaki.NetworkConfig{}
	})

	Describe("Apply", func() {
		var (
			netnsFD *os.File

			existingBridge *net.Interface
		)

		BeforeEach(func() {
			var err error
			netnsFD, err = ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			existingBridge = &net.Interface{Name: "bridge"}
		})

		JustBeforeEach(func() {
			linkConfigurer.InterfaceByNameFunc = func(name string) (*net.Interface, bool, error) {
				if name == "bridge" {
					return existingBridge, true, nil
				}

				return nil, false, nil
			}
		})

		AfterEach(func() {
			Expect(os.Remove(netnsFD.Name())).To(Succeed())
		})

		It("creates a virtual ethernet pair", func() {
			config.HostIntf = "host"
			config.BridgeName = "bridge"
			config.ContainerIntf = "container"
			Expect(configurer.Apply(logger, config, netnsFD)).To(Succeed())

			Expect(vethCreator.CreateCalledWith.HostIfcName).To(Equal("host"))
			Expect(vethCreator.CreateCalledWith.ContainerIfcName).To(Equal("container"))
		})

		Context("when creating the pair fails", func() {
			It("returns a wrapped error", func() {
				config.HostIntf = "host"
				config.BridgeName = "bridge"
				config.ContainerIntf = "container"
				vethCreator.CreateReturns.Err = errors.New("foo bar baz")
				err := configurer.Apply(logger, config, netnsFD)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(&configure.VethPairCreationError{Cause: vethCreator.CreateReturns.Err, HostIfcName: "host", ContainerIfcName: "container"}))
			})
		})

		Context("when creating the pair succeeds", func() {
			BeforeEach(func() {
				vethCreator.CreateReturns.Host = &net.Interface{Name: "the-host"}
				vethCreator.CreateReturns.Container = &net.Interface{Name: "the-container"}
			})

			It("should set mtu on the host interface", func() {
				config.HostIntf = "host"
				config.BridgeName = "bridge"
				config.Mtu = 123
				Expect(configurer.Apply(logger, config, netnsFD)).To(Succeed())

				Expect(linkConfigurer.SetMTUCalledWith.Interface).To(Equal(vethCreator.CreateReturns.Host))
				Expect(linkConfigurer.SetMTUCalledWith.MTU).To(Equal(123))
			})

			Context("When setting the mtu fails", func() {
				It("returns a wrapped error", func() {
					config.HostIntf = "host"
					config.BridgeName = "bridge"
					config.ContainerIntf = "container"
					config.Mtu = 14
					linkConfigurer.SetMTUReturns = errors.New("o no")
					err := configurer.Apply(logger, config, netnsFD)
					Expect(err).To(MatchError(&configure.MTUError{Cause: linkConfigurer.SetMTUReturns, Intf: vethCreator.CreateReturns.Host, MTU: 14}))
				})
			})

			It("should move the container interface in to the container's namespace", func() {
				f, err := ioutil.TempFile("", "")
				Expect(err).NotTo(HaveOccurred())

				config.BridgeName = "bridge"

				Expect(configurer.Apply(logger, config, netnsFD)).To(Succeed())
				Expect(linkConfigurer.SetNsCalledWith.Fd).To(Equal(int(netnsFD.Fd())))

				os.RemoveAll(f.Name())
			})

			Context("When moving the container interface into the namespace fails", func() {
				It("returns a wrapped error", func() {
					config.BridgeName = "bridge"

					linkConfigurer.SetNsReturns = errors.New("o no")

					err := configurer.Apply(logger, config, netnsFD)
					Expect(err).To(MatchError(&configure.SetNsFailedError{Cause: linkConfigurer.SetNsReturns, Intf: vethCreator.CreateReturns.Container, Netns: netnsFD}))
				})
			})

			Describe("adding the host to the bridge", func() {
				Context("when the bridge interface does not exist", func() {
					It("creates the bridge", func() {
						config.BridgeName = "banana-bridge"
						Expect(configurer.Apply(logger, config, netnsFD)).To(Succeed())
						Expect(bridger.CreateCalledWith.Name).To(Equal("banana-bridge"))
					})

					It("adds the host interface to the created bridge", func() {
						createdBridge := &net.Interface{Name: "created"}
						bridger.CreateReturns.Interface = createdBridge

						config.BridgeName = "banana-bridge"
						Expect(configurer.Apply(logger, config, netnsFD)).To(Succeed())
						Expect(bridger.AddCalledWith.Bridge).To(Equal(createdBridge))
					})

					Context("but if creating the bridge fails", func() {
						It("returns an error", func() {
							bridger.CreateReturns.Error = errors.New("kawasaki!")

							config.BridgeName = "banana-bridge"
							Expect(configurer.Apply(logger, config, netnsFD)).To(MatchError("kawasaki!"))
						})
					})
				})

				Context("when the bridge interface exists", func() {
					It("adds the host interface to the existing bridge", func() {
						config.BridgeName = "bridge"
						Expect(configurer.Apply(logger, config, netnsFD)).To(Succeed())
						Expect(bridger.AddCalledWith.Bridge).To(Equal(existingBridge))
					})

					It("brings the host interface up", func() {
						config.BridgeName = "bridge"
						Expect(configurer.Apply(logger, config, netnsFD)).To(Succeed())
						Expect(linkConfigurer.SetUpCalledWith).To(ContainElement(vethCreator.CreateReturns.Host))
					})

					Context("when bringing the host interface up fails", func() {
						It("returns a wrapped error", func() {
							cause := errors.New("there's jam in this sandwich and it's not ok")
							linkConfigurer.SetUpFunc = func(intf *net.Interface) error {
								if vethCreator.CreateReturns.Host == intf {
									return cause
								}

								return nil
							}

							config.BridgeName = "bridge"
							err := configurer.Apply(logger, config, netnsFD)
							Expect(err).To(MatchError(&configure.LinkUpError{Cause: cause, Link: vethCreator.CreateReturns.Host, Role: "host"}))
						})
					})
				})
			})
		})
	})

	Describe("Destroy", func() {
		It("should destroy the bridge", func() {
			config.HostIntf = "host"
			config.BridgeName = "spiderman-bridge"
			Expect(configurer.Destroy(config)).To(Succeed())

			Expect(bridger.DestroyCalledWith).To(HaveLen(1))
			Expect(bridger.DestroyCalledWith[0]).To(Equal(config.BridgeName))
		})

		Context("when bridge fails to be destroyed", func() {
			It("should return an error", func() {
				bridger.DestroyReturns = errors.New("banana-bridge-failure")

				Expect(configurer.Destroy(config)).NotTo(Succeed())
			})
		})
	})
})

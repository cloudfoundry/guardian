package configure_test

import (
	"errors"
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/configure"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/devices/fakedevices"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Container", func() {
	var (
		linkApplyr *fakedevices.FakeLink
		configurer *configure.Container
		config     kawasaki.NetworkConfig
		logger     lager.Logger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		linkApplyr = &fakedevices.FakeLink{AddIPReturns: make(map[string]error)}
		configurer = &configure.Container{
			Link: linkApplyr,
		}
	})

	Context("when the loopback device does not exist", func() {
		var eth *net.Interface
		BeforeEach(func() {
			linkApplyr.InterfaceByNameFunc = func(name string) (*net.Interface, bool, error) {
				if name != "lo" {
					return eth, true, nil
				}

				return nil, false, nil
			}
		})

		It("returns a wrapped error", func() {
			err := configurer.Apply(logger, config)
			Expect(err).To(MatchError(&configure.FindLinkError{Cause: nil, Role: "loopback", Name: "lo"}))
		})

		It("does not attempt to configure other devices", func() {
			Expect(configurer.Apply(logger, config)).ToNot(Succeed())
			Expect(linkApplyr.SetUpCalledWith).ToNot(ContainElement(eth))
		})
	})

	Context("when the loopback exists", func() {
		var lo *net.Interface

		BeforeEach(func() {
			lo = &net.Interface{Name: "lo"}
			linkApplyr.InterfaceByNameFunc = func(name string) (*net.Interface, bool, error) {
				return &net.Interface{Name: name}, true, nil
			}
		})

		It("adds 127.0.0.1/8 as an address", func() {
			ip, subnet, _ := net.ParseCIDR("127.0.0.1/8")
			Expect(configurer.Apply(logger, config)).To(Succeed())
			Expect(linkApplyr.AddIPCalledWith).To(ContainElement(fakedevices.InterfaceIPAndSubnet{Interface: lo, IP: ip, Subnet: subnet}))
		})

		Context("when adding the IP address fails", func() {
			It("returns a wrapped error", func() {
				linkApplyr.AddIPReturns["lo"] = errors.New("o no")
				err := configurer.Apply(logger, config)
				ip, subnet, _ := net.ParseCIDR("127.0.0.1/8")
				Expect(err).To(MatchError(&configure.ConfigureLinkError{Cause: errors.New("o no"), Role: "loopback", Interface: lo, IntendedIP: ip, IntendedSubnet: subnet}))
			})
		})

		It("brings it up", func() {
			Expect(configurer.Apply(logger, config)).To(Succeed())
			Expect(linkApplyr.SetUpCalledWith).To(ContainElement(lo))
		})

		Context("when bringing the link up fails", func() {
			It("returns a wrapped error", func() {
				linkApplyr.SetUpFunc = func(intf *net.Interface) error {
					return errors.New("o no")
				}

				err := configurer.Apply(logger, config)
				Expect(err).To(MatchError(&configure.LinkUpError{Cause: errors.New("o no"), Link: lo, Role: "loopback"}))
			})
		})
	})

	Context("when the container interface does not exist", func() {
		BeforeEach(func() {
			linkApplyr.InterfaceByNameFunc = func(name string) (*net.Interface, bool, error) {
				if name == "lo" {
					return &net.Interface{Name: name}, true, nil
				}

				return nil, false, nil
			}
		})

		It("returns a wrapped error", func() {
			config.ContainerIntf = "foo"
			err := configurer.Apply(logger, config)
			Expect(err).To(MatchError(&configure.FindLinkError{Cause: nil, Role: "container", Name: "foo"}))
		})
	})

	Context("when the container interface exists", func() {
		BeforeEach(func() {
			linkApplyr.InterfaceByNameFunc = func(name string) (*net.Interface, bool, error) {
				return &net.Interface{Name: name}, true, nil
			}
		})

		It("Adds the requested IP", func() {
			config.ContainerIntf = "foo"
			config.ContainerIP, config.Subnet, _ = net.ParseCIDR("2.3.4.5/6")

			Expect(configurer.Apply(logger, config)).To(Succeed())
			Expect(linkApplyr.AddIPCalledWith).To(ContainElement(fakedevices.InterfaceIPAndSubnet{
				Interface: &net.Interface{Name: "foo"},
				IP:        config.ContainerIP,
				Subnet:    config.Subnet,
			}))
		})

		Context("when adding the IP fails", func() {
			It("returns a wrapped error", func() {
				linkApplyr.AddIPReturns["foo"] = errors.New("o no")

				config.ContainerIntf = "foo"
				config.ContainerIP, config.Subnet, _ = net.ParseCIDR("2.3.4.5/6")
				err := configurer.Apply(logger, config)
				Expect(err).To(MatchError(&configure.ConfigureLinkError{
					Cause:          errors.New("o no"),
					Role:           "container",
					Interface:      &net.Interface{Name: "foo"},
					IntendedIP:     config.ContainerIP,
					IntendedSubnet: config.Subnet,
				}))
			})
		})

		It("Brings the link up", func() {
			config.ContainerIntf = "foo"
			Expect(configurer.Apply(logger, config)).To(Succeed())
			Expect(linkApplyr.SetUpCalledWith).To(ContainElement(&net.Interface{Name: "foo"}))
		})

		Context("when bringing the link up fails", func() {
			It("returns a wrapped error", func() {
				cause := errors.New("who ate my pie?")
				linkApplyr.SetUpFunc = func(iface *net.Interface) error {
					if iface.Name == "foo" {
						return cause
					}

					return nil
				}

				config.ContainerIntf = "foo"
				err := configurer.Apply(logger, config)
				Expect(err).To(MatchError(&configure.LinkUpError{Cause: cause, Link: &net.Interface{Name: "foo"}, Role: "container"}))
			})
		})

		It("sets the mtu", func() {
			config.ContainerIntf = "foo"
			config.Mtu = 1234
			Expect(configurer.Apply(logger, config)).To(Succeed())
			Expect(linkApplyr.SetMTUCalledWith.Interface).To(Equal(&net.Interface{Name: "foo"}))
			Expect(linkApplyr.SetMTUCalledWith.MTU).To(Equal(1234))
		})

		Context("when setting the mtu fails", func() {
			It("returns a wrapped error", func() {
				linkApplyr.SetMTUReturns = errors.New("this is NOT the right potato")

				config.ContainerIntf = "foo"
				config.Mtu = 1234
				err := configurer.Apply(logger, config)
				Expect(err).To(MatchError(&configure.MTUError{Cause: linkApplyr.SetMTUReturns, Intf: &net.Interface{Name: "foo"}, MTU: 1234}))
			})
		})

		It("adds a default gateway with the requested IP", func() {
			config.ContainerIntf = "foo"
			config.BridgeIP = net.ParseIP("2.3.4.5")
			Expect(configurer.Apply(logger, config)).To(Succeed())
			Expect(linkApplyr.AddDefaultGWCalledWith.Interface).To(Equal(&net.Interface{Name: "foo"}))
			Expect(linkApplyr.AddDefaultGWCalledWith.IP).To(Equal(net.ParseIP("2.3.4.5")))
		})

		Context("when adding a default gateway fails", func() {
			It("returns a wrapped error", func() {
				linkApplyr.AddDefaultGWReturns = errors.New("this is NOT the right potato")

				config.ContainerIntf = "foo"
				config.BridgeIP = net.ParseIP("2.3.4.5")
				err := configurer.Apply(logger, config)
				Expect(err).To(MatchError(&configure.ConfigureDefaultGWError{Cause: linkApplyr.AddDefaultGWReturns, Interface: &net.Interface{Name: "foo"}, IP: net.ParseIP("2.3.4.5")}))
			})
		})
	})
})

package kawasaki_test

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets/fake_subnet_pool"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Networker", func() {
	var (
		fakeSpecParser     *fakes.FakeSpecParser
		fakeSubnetPool     *fake_subnet_pool.FakePool
		fakeConfigCreator  *fakes.FakeConfigCreator
		fakeConfigurer     *fakes.FakeConfigurer
		fakeConfigStore    *fakes.FakeConfigStore
		fakePortForwarder  *fakes.FakePortForwarder
		fakePortPool       *fakes.FakePortPool
		fakeFirewallOpener *fakes.FakeFirewallOpener
		networker          *kawasaki.Networker
		logger             lager.Logger
		networkConfig      kawasaki.NetworkConfig
		config             map[string]string
	)

	BeforeEach(func() {
		fakeSpecParser = new(fakes.FakeSpecParser)
		fakeSubnetPool = new(fake_subnet_pool.FakePool)
		fakeConfigurer = new(fakes.FakeConfigurer)
		fakeConfigCreator = new(fakes.FakeConfigCreator)
		fakeConfigStore = new(fakes.FakeConfigStore)
		fakePortForwarder = new(fakes.FakePortForwarder)
		fakePortPool = new(fakes.FakePortPool)
		fakeFirewallOpener = new(fakes.FakeFirewallOpener)

		logger = lagertest.NewTestLogger("test")
		networker = kawasaki.New(
			"/path/to/kawasaki",
			fakeSpecParser,
			fakeSubnetPool,
			fakeConfigCreator,
			fakeConfigurer,
			fakeConfigStore,
			fakePortPool,
			fakePortForwarder,
			fakeFirewallOpener,
		)

		ip, subnet, err := net.ParseCIDR("123.123.123.12/24")
		Expect(err).NotTo(HaveOccurred())
		networkConfig = kawasaki.NetworkConfig{
			HostIntf:        "banana-iface",
			ContainerIntf:   "container-of-bananas-iface",
			IPTablePrefix:   "bananas-",
			IPTableInstance: "table",
			BridgeName:      "bananas-bridge",
			BridgeIP:        net.ParseIP("123.123.123.1"),
			ContainerIP:     ip,
			ExternalIP:      net.ParseIP("128.128.90.90"),
			Subnet:          subnet,
			Mtu:             1200,
			DNSServers: []net.IP{
				net.ParseIP("8.8.8.8"),
				net.ParseIP("8.8.4.4"),
			},
		}

		fakeConfigCreator.CreateReturns(networkConfig, nil)

		config = map[string]string{
			gardener.ContainerIPKey:        networkConfig.ContainerIP.String(),
			"kawasaki.host-interface":      networkConfig.HostIntf,
			"kawasaki.container-interface": networkConfig.ContainerIntf,
			"kawasaki.bridge-interface":    networkConfig.BridgeName,
			gardener.BridgeIPKey:           networkConfig.BridgeIP.String(),
			gardener.ExternalIPKey:         networkConfig.ExternalIP.String(),
			"kawasaki.subnet":              networkConfig.Subnet.String(),
			"kawasaki.iptable-prefix":      networkConfig.IPTablePrefix,
			"kawasaki.iptable-inst":        networkConfig.IPTableInstance,
			"kawasaki.mtu":                 strconv.Itoa(networkConfig.Mtu),
			"kawasaki.dns-servers":         "8.8.8.8, 8.8.4.4",
		}

		fakeConfigStore.GetStub = func(handle, name string) (string, error) {
			Expect(handle).To(Equal("some-handle"))
			return config[name], nil
		}
	})

	Describe("Hook", func() {
		It("parses the spec", func() {
			networker.Hooks(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeSpecParser.ParseCallCount()).To(Equal(1))
			_, spec := fakeSpecParser.ParseArgsForCall(0)
			Expect(spec).To(Equal("1.2.3.4/30"))
		})

		It("returns an error if the spec can't be parsed", func() {
			fakeSpecParser.ParseReturns(nil, nil, errors.New("no parsey"))
			_, err := networker.Hooks(logger, "some-handle", "1.2.3.4/30")
			Expect(err).To(MatchError("no parsey"))
		})

		It("acquires a subnet and IP", func() {
			someSubnetRequest := subnets.DynamicSubnetSelector
			someIpRequest := subnets.DynamicIPSelector
			fakeSpecParser.ParseReturns(someSubnetRequest, someIpRequest, nil)

			networker.Hooks(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeSubnetPool.AcquireCallCount()).To(Equal(1))
			_, sr, ir := fakeSubnetPool.AcquireArgsForCall(0)
			Expect(sr).To(Equal(someSubnetRequest))
			Expect(ir).To(Equal(someIpRequest))
		})

		It("creates a network config", func() {
			someIp, someSubnet, err := net.ParseCIDR("1.2.3.4/5")
			fakeSubnetPool.AcquireReturns(someSubnet, someIp, err)

			networker.Hooks(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeConfigCreator.CreateCallCount()).To(Equal(1))
			_, handle, subnet, ip := fakeConfigCreator.CreateArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(subnet).To(Equal(someSubnet))
			Expect(ip).To(Equal(someIp))
		})

		It("stores the config to ConfigStore", func() {
			config := make(map[string]string)
			fakeConfigStore.SetStub = func(handle, name, value string) {
				Expect(handle).To(Equal("some-handle"))

				config[name] = value
			}

			_, err := networker.Hooks(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(config["kawasaki.host-interface"]).To(Equal(networkConfig.HostIntf))
			Expect(config["kawasaki.container-interface"]).To(Equal(networkConfig.ContainerIntf))
			Expect(config["kawasaki.bridge-interface"]).To(Equal(networkConfig.BridgeName))
			Expect(config[gardener.BridgeIPKey]).To(Equal(networkConfig.BridgeIP.String()))
			Expect(config[gardener.ContainerIPKey]).To(Equal(networkConfig.ContainerIP.String()))
			Expect(config[gardener.ExternalIPKey]).To(Equal(networkConfig.ExternalIP.String()))
			Expect(config["kawasaki.subnet"]).To(Equal(networkConfig.Subnet.String()))
			Expect(config["kawasaki.iptable-prefix"]).To(Equal(networkConfig.IPTablePrefix))
			Expect(config["kawasaki.iptable-inst"]).To(Equal(networkConfig.IPTableInstance))
			Expect(config["kawasaki.mtu"]).To(Equal(strconv.Itoa(networkConfig.Mtu)))
			Expect(config["kawasaki.dns-servers"]).To(Equal("8.8.8.8, 8.8.4.4"))
		})

		Context("when the configuration can't be created", func() {
			It("returns a wrapped error", func() {
				fakeConfigCreator.CreateReturns(kawasaki.NetworkConfig{}, errors.New("bad config"))
				_, err := networker.Hooks(logger, "some-handle", "1.2.3.4/30")
				Expect(err).To(MatchError("create network config: bad config"))
			})
		})

		It("returns the path to the kawasaki binary with the created config as flags", func() {
			hooks, err := networker.Hooks(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(hooks.Prestart.Path).To(Equal("/path/to/kawasaki"))
		})

		It("passes the config as flags to the binary", func() {
			hooks, err := networker.Hooks(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(hooks.Prestart.Args).To(ContainElement("--host-interface=" + networkConfig.HostIntf))
			Expect(hooks.Prestart.Args).To(ContainElement("--container-interface=" + networkConfig.ContainerIntf))
			Expect(hooks.Prestart.Args).To(ContainElement("--bridge-interface=" + networkConfig.BridgeName))
			Expect(hooks.Prestart.Args).To(ContainElement("--bridge-ip=" + networkConfig.BridgeIP.String()))
			Expect(hooks.Prestart.Args).To(ContainElement("--container-ip=" + networkConfig.ContainerIP.String()))
			Expect(hooks.Prestart.Args).To(ContainElement("--external-ip=" + networkConfig.ExternalIP.String()))
			Expect(hooks.Prestart.Args).To(ContainElement("--subnet=" + networkConfig.Subnet.String()))
			Expect(hooks.Prestart.Args).To(ContainElement("--iptable-instance=" + networkConfig.IPTableInstance))
			Expect(hooks.Prestart.Args).To(ContainElement("--iptable-prefix=" + networkConfig.IPTablePrefix))
			Expect(hooks.Prestart.Args).To(ContainElement("--mtu=" + strconv.Itoa(networkConfig.Mtu)))
			for _, dnsServer := range networkConfig.DNSServers {
				Expect(hooks.Prestart.Args).To(ContainElement("--dns-server=" + dnsServer.String()))
			}
		})
	})

	Describe("Capacity", func() {
		BeforeEach(func() {
			fakeSubnetPool.CapacityReturns(9000)
		})

		It("delegates to subnetPool for capacity", func() {
			cap := networker.Capacity()

			Expect(fakeSubnetPool.CapacityCallCount()).To(Equal(1))
			Expect(cap).To(BeEquivalentTo(9000))
		})
	})

	Describe("Destroy", func() {
		It("should destroy the configuration", func() {
			Expect(networker.Destroy(logger, "some-handle")).To(Succeed())

			Expect(fakeConfigurer.DestroyCallCount()).To(Equal(1))
			_, netConfig := fakeConfigurer.DestroyArgsForCall(0)
			Expect(netConfig).To(Equal(networkConfig))
		})

		Context("when the configuration is not destroyed", func() {
			It("should return the error", func() {
				fakeConfigurer.DestroyReturns(errors.New("spiderman-error"))

				err := networker.Destroy(logger, "some-handle")
				Expect(err).To(MatchError("spiderman-error"))
			})
		})

		It("releases the subnet", func() {
			Expect(networker.Destroy(logger, "some-handle")).To(Succeed())

			Expect(fakeSubnetPool.ReleaseCallCount()).To(Equal(1))
			actualSubnet, actualIp := fakeSubnetPool.ReleaseArgsForCall(0)

			Expect(actualIp).To(Equal(networkConfig.ContainerIP))
			Expect(actualSubnet).To(Equal(networkConfig.Subnet))
		})

		Context("when releasing subnet fails", func() {
			It("should return the error", func() {
				fakeSubnetPool.ReleaseReturns(errors.New("oh no"))

				Expect(networker.Destroy(logger, "some-handle")).To(MatchError("oh no"))
			})
		})
	})

	Describe("NetOut", func() {
		It("delegates to FirewallOpener", func() {
			rule := garden.NetOutRule{Protocol: garden.ProtocolICMP}

			fakeFirewallOpener.OpenReturns(errors.New("potato"))
			Expect(networker.NetOut(lagertest.NewTestLogger(""), "some-handle", rule)).To(MatchError("potato"))

			_, chainArg, ruleArg := fakeFirewallOpener.OpenArgsForCall(0)
			Expect(chainArg).To(Equal(networkConfig.IPTableInstance))
			Expect(ruleArg).To(Equal(rule))
		})
	})

	Describe("NetIn", func() {
		var (
			externalPort  uint32
			containerPort uint32
			handle        string
		)

		BeforeEach(func() {
			externalPort = 123
			containerPort = 456

			handle = "some-handle"
		})

		It("calls the PortForwarder with correct parameters", func() {
			_, _, err := networker.NetIn(logger, handle, externalPort, containerPort)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakePortForwarder.ForwardCallCount()).To(Equal(1))

			actualSpec := fakePortForwarder.ForwardArgsForCall(0)
			Expect(actualSpec.InstanceID).To(Equal(networkConfig.IPTableInstance))
			Expect(actualSpec.ContainerIP).To(Equal(networkConfig.ContainerIP))
			Expect(actualSpec.ExternalIP).To(Equal(networkConfig.ExternalIP))
			Expect(actualSpec.FromPort).To(Equal(externalPort))
			Expect(actualSpec.ToPort).To(Equal(containerPort))

			Expect(fakePortPool.AcquireCallCount()).To(Equal(0))
		})

		Context("when external port is not specified", func() {
			It("acquires a random port from the pool", func() {
				fakePortPool.AcquireReturns(externalPort, nil)

				actualHostPort, actualContainerPort, err := networker.NetIn(logger, handle, 0, containerPort)
				Expect(err).NotTo(HaveOccurred())

				Expect(actualHostPort).To(Equal(externalPort))
				Expect(actualContainerPort).To(Equal(containerPort))

				Expect(fakePortPool.AcquireCallCount()).To(Equal(1))
				Expect(fakePortForwarder.ForwardCallCount()).To(Equal(1))
				spec := fakePortForwarder.ForwardArgsForCall(0)

				Expect(spec.FromPort).To(Equal(externalPort))
				Expect(spec.ToPort).To(Equal(containerPort))
			})
		})

		Context("when port pool fails to acquire", func() {
			var err error

			BeforeEach(func() {
				fakePortPool.AcquireReturns(0, fmt.Errorf("Oh no!"))
				_, _, err = networker.NetIn(logger, handle, 0, containerPort)
			})

			It("returns the error", func() {
				Expect(err).To(MatchError("Oh no!"))
			})

			It("does not add a new port mapping", func() {
				Expect(fakeConfigStore.SetCallCount()).To(Equal(0))
			})

			It("does not do port forwarding", func() {
				Expect(fakePortForwarder.ForwardCallCount()).To(Equal(0))
			})
		})

		Context("when container port is not specified", func() {
			It("aquires a port from the pool", func() {
				actualHostPort, actualContainerPort, err := networker.NetIn(logger, handle, externalPort, 0)
				Expect(err).ToNot(HaveOccurred())

				Expect(actualHostPort).To(Equal(externalPort))
				Expect(actualContainerPort).To(Equal(externalPort))
			})
		})

		It("stores port mapping in ConfigStore", func() {
			_, _, err := networker.NetIn(logger, handle, externalPort, containerPort)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConfigStore.SetCallCount()).To(Equal(1))

			actualHandle, actualName, actualValue := fakeConfigStore.SetArgsForCall(0)
			Expect(actualHandle).To(Equal(handle))
			Expect(actualName).To(Equal(gardener.MappedPortsKey))
			Expect(actualValue).To(Equal(`[{"HostPort":123,"ContainerPort":456}]`))
		})

		It("stores a list of port mappings in ConfigStore", func() {
			_, _, err := networker.NetIn(logger, handle, externalPort, containerPort)
			Expect(err).NotTo(HaveOccurred())

			config[gardener.MappedPortsKey] = `[{"HostPort":123,"ContainerPort":456}]`

			_, _, err = networker.NetIn(logger, handle, 654, 987)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConfigStore.SetCallCount()).To(Equal(2))

			_, _, actualValue := fakeConfigStore.SetArgsForCall(1)
			Expect(actualValue).To(Equal(`[{"HostPort":123,"ContainerPort":456},{"HostPort":654,"ContainerPort":987}]`))
		})

		Context("when the PortForwarder fails", func() {
			var err error

			BeforeEach(func() {
				fakePortForwarder.ForwardReturns(fmt.Errorf("Oh no!"))
				_, _, err = networker.NetIn(logger, handle, 0, 0)
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("Oh no!"))
			})

			It("does not add the new port mapping", func() {
				Expect(fakeConfigStore.SetCallCount()).To(Equal(0))
			})
		})

		Context("when handle does not exist", func() {
			BeforeEach(func() {
				fakeConfigStore.GetReturns("", errors.New("Handle does not exist"))
			})

			It("returns an error", func() {
				_, _, err := networker.NetIn(logger, "nonexistent", 0, 0)
				Expect(err).To(MatchError("Handle does not exist"))
			})
		})
	})
})

package kawasaki_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/kawasaki"
	fakes "code.cloudfoundry.org/guardian/kawasaki/kawasakifakes"
	"code.cloudfoundry.org/guardian/kawasaki/subnets"
	"code.cloudfoundry.org/guardian/kawasaki/subnets/fake_subnet_pool"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Networker", func() {
	var (
		fakeSpecParser     *fakes.FakeSpecParser
		fakeSubnetPool     *fake_subnet_pool.FakePool
		fakeConfigCreator  *fakes.FakeConfigCreator
		fakeConfigStore    *fakes.FakeConfigStore
		fakePortForwarder  *fakes.FakePortForwarder
		fakePortPool       *fakes.FakePortPool
		fakeFirewallOpener *fakes.FakeFirewallOpener
		fakeConfigurer     *fakes.FakeConfigurer
		containerSpec      garden.ContainerSpec
		networker          kawasaki.Networker
		logger             lager.Logger
		networkConfig      kawasaki.NetworkConfig
		config             map[string]string
	)

	BeforeEach(func() {
		fakeSpecParser = new(fakes.FakeSpecParser)
		fakeSubnetPool = new(fake_subnet_pool.FakePool)
		fakeConfigCreator = new(fakes.FakeConfigCreator)
		fakeConfigStore = new(fakes.FakeConfigStore)
		fakePortForwarder = new(fakes.FakePortForwarder)
		fakePortPool = new(fakes.FakePortPool)
		fakeFirewallOpener = new(fakes.FakeFirewallOpener)
		fakeConfigurer = new(fakes.FakeConfigurer)

		containerSpec = garden.ContainerSpec{
			Handle:  "some-handle",
			Network: "1.2.3.4/30",
		}

		logger = lagertest.NewTestLogger("test")
		networker = kawasaki.New(
			"/sbin/iptables",
			fakeSpecParser,
			fakeSubnetPool,
			fakeConfigCreator,
			fakeConfigStore,
			fakeConfigurer,
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

		portMappings, err := json.Marshal([]garden.PortMapping{
			garden.PortMapping{
				HostPort:      60000,
				ContainerPort: 8080,
			},
		})
		Expect(err).NotTo(HaveOccurred())

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
			gardener.MappedPortsKey:        string(portMappings),
		}

		fakeConfigStore.GetStub = func(handle, name string) (string, bool) {
			Expect(handle).To(Equal("some-handle"))
			if val, ok := config[name]; ok {
				return val, true
			}

			return "", false
		}
	})

	Describe("Network", func() {
		It("parses the spec", func() {
			networker.Network(logger, containerSpec, 42)
			Expect(fakeSpecParser.ParseCallCount()).To(Equal(1))
			_, spec := fakeSpecParser.ParseArgsForCall(0)
			Expect(spec).To(Equal("1.2.3.4/30"))
		})

		It("returns an error if the spec can't be parsed", func() {
			fakeSpecParser.ParseReturns(nil, nil, errors.New("no parsey"))
			err := networker.Network(logger, containerSpec, 42)
			Expect(err).To(MatchError("no parsey"))
		})

		It("acquires a subnet and IP", func() {
			someSubnetRequest := subnets.DynamicSubnetSelector
			someIpRequest := subnets.DynamicIPSelector
			fakeSpecParser.ParseReturns(someSubnetRequest, someIpRequest, nil)

			networker.Network(logger, containerSpec, 42)
			Expect(fakeSubnetPool.AcquireCallCount()).To(Equal(1))
			_, sr, ir := fakeSubnetPool.AcquireArgsForCall(0)
			Expect(sr).To(Equal(someSubnetRequest))
			Expect(ir).To(Equal(someIpRequest))
		})

		It("creates a network config", func() {
			someIp, someSubnet, err := net.ParseCIDR("1.2.3.4/5")
			fakeSubnetPool.AcquireReturns(someSubnet, someIp, err)

			networker.Network(logger, containerSpec, 42)
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

			err := networker.Network(logger, containerSpec, 42)
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

		It("applies the right configuration", func() {
			err := networker.Network(logger, containerSpec, 42)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConfigurer.ApplyCallCount()).To(Equal(1))
			_, actualNetConfig, pid := fakeConfigurer.ApplyArgsForCall(0)
			Expect(err).NotTo(HaveOccurred())
			Expect(actualNetConfig).To(Equal(networkConfig))
			Expect(pid).To(Equal(42))
		})

		Context("when the configurer fails to apply the config", func() {
			It("errors", func() {
				fakeConfigurer.ApplyReturns(errors.New("wont-apply"))
				Expect(networker.Network(logger, containerSpec, 42)).To(MatchError("wont-apply"))
			})
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
		Context("when the store does not contain the properties for the container", func() {
			It("should skip destroy, to maintain idempotence", func() {
				config = nil
				Expect(networker.Destroy(logger, "some-handle")).To(Succeed())
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
			Context("when the error indicates the subnet is already gone", func() {
				It("should return nil (no error)", func() {
					fakeSubnetPool.ReleaseReturns(subnets.ErrReleasedUnallocatedSubnet)
					Expect(networker.Destroy(logger, "some-handle")).To(BeNil())
				})
			})

			Context("when the error is generic", func() {
				It("should return the error", func() {
					fakeSubnetPool.ReleaseReturns(errors.New("oh no"))
					Expect(networker.Destroy(logger, "some-handle")).To(MatchError("oh no"))
				})
			})
		})

		Describe("releasing ports", func() {
			Context("when there are no ports stored in the config", func() {
				BeforeEach(func() {
					delete(config, gardener.MappedPortsKey)
				})

				It("does nothing when no ports are stored in the config", func() {
					Expect(networker.Destroy(logger, "some-handle")).To(Succeed())
					Expect(fakePortPool.ReleaseCallCount()).To(Equal(0))
				})
			})

			It("destroys any ports named in the config", func() {
				config[gardener.MappedPortsKey] = `[{"HostPort": 123}, {"HostPort": 456}]`

				Expect(networker.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakePortPool.ReleaseCallCount()).To(Equal(2))
				Expect(fakePortPool.ReleaseArgsForCall(0)).To(BeEquivalentTo(123))
				Expect(fakePortPool.ReleaseArgsForCall(1)).To(BeEquivalentTo(456))
			})

			It("returns an error if the ports property is not valid JSON", func() {
				config[gardener.MappedPortsKey] = `potato`
				Expect(networker.Destroy(logger, "some-handle")).To(MatchError(ContainSubstring("invalid")))
			})
		})

		Describe("destroying network configuration", func() {
			It("destroys iptables rules", func() {
				Expect(networker.Destroy(logger, "some-handle")).To(Succeed())
				Expect(fakeConfigurer.DestroyIPTablesRulesCallCount()).To(Equal(1))
				_, actualNetCfg := fakeConfigurer.DestroyIPTablesRulesArgsForCall(0)
				Expect(actualNetCfg).To(Equal(networkConfig))
			})

			Context("when configurer fails to destroy iptables rules", func() {
				It("errors", func() {
					fakeConfigurer.DestroyIPTablesRulesReturns(errors.New("boom"))
					Expect(networker.Destroy(logger, "some-handle")).To(MatchError("boom"))
				})
			})

			Context("when the subnet pool has allocated an IP from the subnet", func() {
				BeforeEach(func() {
					fakeSubnetPool.RunIfFreeStub = func(_ *net.IPNet, _ func() error) error {
						return nil
					}
				})

				It("doesn't destroy the bridge", func() {
					Expect(networker.Destroy(logger, "some-handle")).To(Succeed())
					Expect(fakeConfigurer.DestroyBridgeCallCount()).To(Equal(0))
				})
			})

			Context("when the subnet pool has released all IPs from the subnet", func() {
				BeforeEach(func() {
					fakeSubnetPool.RunIfFreeStub = func(_ *net.IPNet, cb func() error) error {
						cb()
						return nil
					}
				})

				It("destroys all network configuration", func() {
					Expect(networker.Destroy(logger, "some-handle")).To(Succeed())
					Expect(fakeConfigurer.DestroyBridgeCallCount()).To(Equal(1))

					loggerArg, networkConfigArg := fakeConfigurer.DestroyBridgeArgsForCall(0)
					Expect(loggerArg).To(Equal(logger))
					Expect(networkConfigArg).To(Equal(networkConfig))
				})

				Context("when the DestroyBridge func returns an error", func() {
					It("returns the error", func() {
						fakeSubnetPool.RunIfFreeReturns(errors.New("error-destroying-bridge"))

						err := networker.Destroy(logger, "some-handle")
						Expect(err).To(MatchError("error-destroying-bridge"))
					})
				})
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
			Expect(actualValue).To(Equal(`[{"HostPort":60000,"ContainerPort":8080},{"HostPort":123,"ContainerPort":456}]`))
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
				fakeConfigStore.GetReturns("", false)
			})

			It("returns an error", func() {
				_, _, err := networker.NetIn(logger, "nonexistent", 0, 0)
				Expect(err).To(MatchError(ContainSubstring("property not found")))
			})
		})
	})

	Describe("Restore", func() {
		It("removes the subnet from the the subnet pool", func() {
			Expect(networker.Restore(logger, "some-handle")).To(Succeed())
			Expect(fakeSubnetPool.RemoveCallCount()).To(Equal(1))
			calledSubnet, calledContainerIP := fakeSubnetPool.RemoveArgsForCall(0)
			Expect(calledSubnet.String()).To(Equal("123.123.123.0/24"))
			Expect(calledContainerIP.String()).To(Equal("123.123.123.12"))
		})

		It("removes the port from port mapping list", func() {
			Expect(networker.Restore(logger, "some-handle")).To(Succeed())
			Expect(fakePortPool.RemoveCallCount()).To(Equal(1))
			calledPort := fakePortPool.RemoveArgsForCall(0)
			Expect(calledPort).To(BeEquivalentTo(60000))
		})

		Context("when the config couldn't be loaded", func() {
			It("returns an appropriate error", func() {
				config = nil
				Expect(networker.Restore(logger, "some-handle")).To(MatchError(ContainSubstring("loading some-handle")))
			})
		})

		Context("when removing the IP from the subnet pool errors", func() {
			BeforeEach(func() {
				fakeSubnetPool.RemoveReturns(errors.New("failed-to-remove-from-subnet-pool"))
			})
			It("returns an appropriate error", func() {
				Expect(networker.Restore(logger, "some-handle")).To(MatchError("subnet pool removing some-handle: failed-to-remove-from-subnet-pool"))
			})
		})

		Context("when there are no port mappings", func() {
			BeforeEach(func() {
				delete(config, gardener.MappedPortsKey)
			})

			It("completes successfully", func() {
				Expect(networker.Restore(logger, "some-handle")).To(Succeed())
			})
		})

		Context("when the port mapping json can't be marshaled", func() {
			BeforeEach(func() {
				config[gardener.MappedPortsKey] = "not-json"
			})

			It("returns an appropriate erorr", func() {
				Expect(networker.Restore(logger, "some-handle")).To(MatchError("unmarshaling port mappings some-handle: invalid character 'o' in literal null (expecting 'u')"))
			})
		})

		Context("when removing the port from the port pool errors", func() {
			BeforeEach(func() {
				fakePortPool.RemoveReturns(errors.New("failed-to-remove-from-port-pool"))
			})
			It("returns an appropriate error", func() {
				Expect(networker.Restore(logger, "some-handle")).To(MatchError("port pool removing some-handle: failed-to-remove-from-port-pool"))
			})
		})
	})
})

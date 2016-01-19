package kawasaki_test

import (
	"errors"
	"fmt"
	"net"
	"strconv"

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
		fakeSpecParser    *fakes.FakeSpecParser
		fakeSubnetPool    *fake_subnet_pool.FakePool
		fakeConfigCreator *fakes.FakeConfigCreator
		fakeConfigurer    *fakes.FakeConfigurer
		fakeConfigStore   *fakes.FakeConfigStore
		fakePortForwarder *fakes.FakePortForwarder
		fakePortPool      *fakes.FakePortPool
		networker         *kawasaki.Networker
		logger            lager.Logger
		networkConfig     kawasaki.NetworkConfig
	)

	BeforeEach(func() {
		fakeSpecParser = new(fakes.FakeSpecParser)
		fakeSubnetPool = new(fake_subnet_pool.FakePool)
		fakeConfigurer = new(fakes.FakeConfigurer)
		fakeConfigCreator = new(fakes.FakeConfigCreator)
		fakeConfigStore = new(fakes.FakeConfigStore)
		fakePortForwarder = new(fakes.FakePortForwarder)
		fakePortPool = new(fakes.FakePortPool)

		logger = lagertest.NewTestLogger("test")
		networker = kawasaki.New(
			"/path/to/kawasaki",
			fakeSpecParser,
			fakeSubnetPool,
			fakeConfigCreator,
			fakeConfigurer,
			fakeConfigStore,
			fakePortForwarder,
			fakePortPool,
			"potato",
		)

		ip, subnet, err := net.ParseCIDR("123.123.123.12/24")
		Expect(err).NotTo(HaveOccurred())
		networkConfig = kawasaki.NetworkConfig{
			HostIntf:      "banana-iface",
			ContainerIntf: "container-of-bananas-iface",
			IPTableChain:  "bananas-table",
			BridgeName:    "bananas-bridge",
			BridgeIP:      net.ParseIP("123.123.123.1"),
			ContainerIP:   ip,
			ExternalIP:    net.ParseIP("128.128.90.90"),
			Subnet:        subnet,
			Mtu:           1200,
		}

		fakeConfigCreator.CreateReturns(networkConfig, nil)

		config := map[string]string{
			"kawasaki.host-interface":      networkConfig.HostIntf,
			"kawasaki.container-interface": networkConfig.ContainerIntf,
			"kawasaki.bridge-interface":    networkConfig.BridgeName,
			"kawasaki.bridge-ip":           networkConfig.BridgeIP.String(),
			"kawasaki.container-ip":        networkConfig.ContainerIP.String(),
			"kawasaki.external-ip":         networkConfig.ExternalIP.String(),
			"kawasaki.subnet":              networkConfig.Subnet.String(),
			"kawasaki.iptable-chain":       networkConfig.IPTableChain,
			"kawasaki.mtu":                 strconv.Itoa(networkConfig.Mtu),
		}

		fakeConfigStore.GetStub = func(handle, name string) (string, error) {
			Expect(handle).To(Equal("some-handle"))
			return config[name], nil
		}
	})

	Describe("Hook", func() {
		It("parses the spec", func() {
			networker.Hook(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeSpecParser.ParseCallCount()).To(Equal(1))
			_, spec := fakeSpecParser.ParseArgsForCall(0)
			Expect(spec).To(Equal("1.2.3.4/30"))
		})

		It("returns an error if the spec can't be parsed", func() {
			fakeSpecParser.ParseReturns(nil, nil, errors.New("no parsey"))
			_, err := networker.Hook(logger, "some-handle", "1.2.3.4/30")
			Expect(err).To(MatchError("no parsey"))
		})

		It("acquires a subnet and IP", func() {
			someSubnetRequest := subnets.DynamicSubnetSelector
			someIpRequest := subnets.DynamicIPSelector
			fakeSpecParser.ParseReturns(someSubnetRequest, someIpRequest, nil)

			networker.Hook(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeSubnetPool.AcquireCallCount()).To(Equal(1))
			_, sr, ir := fakeSubnetPool.AcquireArgsForCall(0)
			Expect(sr).To(Equal(someSubnetRequest))
			Expect(ir).To(Equal(someIpRequest))
		})

		It("creates a network config", func() {
			someIp, someSubnet, err := net.ParseCIDR("1.2.3.4/5")
			fakeSubnetPool.AcquireReturns(someSubnet, someIp, err)

			networker.Hook(logger, "some-handle", "1.2.3.4/30")
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

			_, err := networker.Hook(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(config["kawasaki.host-interface"]).To(Equal(networkConfig.HostIntf))
			Expect(config["kawasaki.container-interface"]).To(Equal(networkConfig.ContainerIntf))
			Expect(config["kawasaki.bridge-interface"]).To(Equal(networkConfig.BridgeName))
			Expect(config["kawasaki.bridge-ip"]).To(Equal(networkConfig.BridgeIP.String()))
			Expect(config["kawasaki.container-ip"]).To(Equal(networkConfig.ContainerIP.String()))
			Expect(config["kawasaki.external-ip"]).To(Equal(networkConfig.ExternalIP.String()))
			Expect(config["kawasaki.subnet"]).To(Equal(networkConfig.Subnet.String()))
			Expect(config["kawasaki.iptable-chain"]).To(Equal(networkConfig.IPTableChain))
			Expect(config["kawasaki.mtu"]).To(Equal(strconv.Itoa(networkConfig.Mtu)))
		})

		Context("when the configuration can't be created", func() {
			It("returns a wrapped error", func() {
				fakeConfigCreator.CreateReturns(kawasaki.NetworkConfig{}, errors.New("bad config"))
				_, err := networker.Hook(logger, "some-handle", "1.2.3.4/30")
				Expect(err).To(MatchError("create network config: bad config"))
			})
		})

		It("returns the path to the kawasaki binary with the created config as flags", func() {
			hook, err := networker.Hook(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(hook.Path).To(Equal("/path/to/kawasaki"))
		})

		It("passes the config as flags to the binary", func() {
			hook, err := networker.Hook(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(hook.Args).To(ContainElement("--host-interface=" + networkConfig.HostIntf))
			Expect(hook.Args).To(ContainElement("--container-interface=" + networkConfig.ContainerIntf))
			Expect(hook.Args).To(ContainElement("--bridge-interface=" + networkConfig.BridgeName))
			Expect(hook.Args).To(ContainElement("--bridge-ip=" + networkConfig.BridgeIP.String()))
			Expect(hook.Args).To(ContainElement("--container-ip=" + networkConfig.ContainerIP.String()))
			Expect(hook.Args).To(ContainElement("--external-ip=" + networkConfig.ExternalIP.String()))
			Expect(hook.Args).To(ContainElement("--subnet=" + networkConfig.Subnet.String()))
			Expect(hook.Args).To(ContainElement("--iptable-chain=" + networkConfig.IPTableChain))
			Expect(hook.Args).To(ContainElement("--mtu=" + strconv.Itoa(networkConfig.Mtu)))
			Expect(hook.Args).To(ContainElement("--tag=potato"))
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
			_, _, err := networker.NetIn(handle, externalPort, containerPort)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakePortForwarder.ForwardCallCount()).To(Equal(1))

			actualSpec := fakePortForwarder.ForwardArgsForCall(0)
			Expect(actualSpec.IPTableChain).To(Equal(networkConfig.IPTableChain))
			Expect(actualSpec.ContainerIP).To(Equal(networkConfig.ContainerIP))
			Expect(actualSpec.ExternalIP).To(Equal(networkConfig.ExternalIP))
			Expect(actualSpec.FromPort).To(Equal(externalPort))
			Expect(actualSpec.ToPort).To(Equal(containerPort))

			Expect(fakePortPool.AcquireCallCount()).To(Equal(0))
		})

		Context("when external port is not specified", func() {
			It("acquires a random port from the pool", func() {
				fakePortPool.AcquireReturns(externalPort, nil)

				actualHostPort, actualContainerPort, err := networker.NetIn(handle, 0, containerPort)
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
			It("returns the error", func() {
				fakePortPool.AcquireReturns(0, fmt.Errorf("Oh no!"))

				actualHostPort, actualContainerPort, err := networker.NetIn(handle, 0, containerPort)
				Expect(err).To(MatchError("Oh no!"))
				Expect(actualHostPort).To(Equal(uint32(0)))
				Expect(actualContainerPort).To(Equal(uint32(0)))

				Expect(fakePortForwarder.ForwardCallCount()).To(Equal(0))
			})
		})

		Context("when container port is not specified", func() {
			It("aquires a port from the pool", func() {
				actualHostPort, actualContainerPort, err := networker.NetIn(handle, externalPort, 0)
				Expect(err).ToNot(HaveOccurred())

				Expect(actualHostPort).To(Equal(externalPort))
				Expect(actualContainerPort).To(Equal(externalPort))
			})
		})

		Context("when the PortForwarder fails", func() {
			It("returns an error", func() {
				fakePortForwarder.ForwardReturns(fmt.Errorf("Oh no!"))

				_, _, err := networker.NetIn(handle, 0, 0)
				Expect(err).To(MatchError("Oh no!"))
			})
		})

		Context("when handle does not exist", func() {
			BeforeEach(func() {
				fakeConfigStore.GetReturns("", errors.New("Handle does not exist"))
			})

			It("returns an error", func() {
				_, _, err := networker.NetIn("nonexistent", 0, 0)
				Expect(err).To(MatchError("Handle does not exist"))
			})
		})
	})
})

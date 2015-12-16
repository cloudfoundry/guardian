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
		fakeNetnsMgr      *fakes.FakeNetnsMgr
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
		fakeNetnsMgr = new(fakes.FakeNetnsMgr)
		fakeSpecParser = new(fakes.FakeSpecParser)
		fakeSubnetPool = new(fake_subnet_pool.FakePool)
		fakeConfigurer = new(fakes.FakeConfigurer)
		fakeConfigCreator = new(fakes.FakeConfigCreator)
		fakeConfigStore = new(fakes.FakeConfigStore)
		fakePortForwarder = new(fakes.FakePortForwarder)
		fakePortPool = new(fakes.FakePortPool)

		logger = lagertest.NewTestLogger("test")
		networker = kawasaki.New(
			fakeNetnsMgr,
			fakeSpecParser,
			fakeSubnetPool,
			fakeConfigCreator,
			fakeConfigurer,
			fakeConfigStore,
			fakePortForwarder,
			fakePortPool,
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
			"kawasaki.subnet-ip":           networkConfig.Subnet.String(),
			"kawasaki.iptable-chain":       networkConfig.IPTableChain,
			"kawasaki.mtu":                 strconv.Itoa(networkConfig.Mtu),
		}

		fakeConfigStore.GetStub = func(handle, name string) (string, error) {
			Expect(handle).To(Equal("some-handle"))
			return config[name], nil
		}
	})

	Describe("Network", func() {
		It("parses the spec", func() {
			networker.Network(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeSpecParser.ParseCallCount()).To(Equal(1))
			_, spec := fakeSpecParser.ParseArgsForCall(0)
			Expect(spec).To(Equal("1.2.3.4/30"))
		})

		It("returns an error if the spec can't be parsed", func() {
			fakeSpecParser.ParseReturns(nil, nil, errors.New("no parsey"))
			_, err := networker.Network(logger, "some-handle", "1.2.3.4/30")
			Expect(err).To(MatchError("no parsey"))
		})

		It("acquires a subnet and IP", func() {
			someSubnetRequest := subnets.DynamicSubnetSelector
			someIpRequest := subnets.DynamicIPSelector
			fakeSpecParser.ParseReturns(someSubnetRequest, someIpRequest, nil)

			networker.Network(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeSubnetPool.AcquireCallCount()).To(Equal(1))
			_, sr, ir := fakeSubnetPool.AcquireArgsForCall(0)
			Expect(sr).To(Equal(someSubnetRequest))
			Expect(ir).To(Equal(someIpRequest))
		})

		It("creates a network config", func() {
			someIp, someSubnet, err := net.ParseCIDR("1.2.3.4/5")
			fakeSubnetPool.AcquireReturns(someSubnet, someIp, err)

			networker.Network(logger, "some-handle", "1.2.3.4/30")
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

			_, err := networker.Network(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(config["kawasaki.host-interface"]).To(Equal(networkConfig.HostIntf))
			Expect(config["kawasaki.container-interface"]).To(Equal(networkConfig.ContainerIntf))
			Expect(config["kawasaki.bridge-interface"]).To(Equal(networkConfig.BridgeName))
			Expect(config["kawasaki.bridge-ip"]).To(Equal(networkConfig.BridgeIP.String()))
			Expect(config["kawasaki.container-ip"]).To(Equal(networkConfig.ContainerIP.String()))
			Expect(config["kawasaki.external-ip"]).To(Equal(networkConfig.ExternalIP.String()))
			Expect(config["kawasaki.subnet-ip"]).To(Equal(networkConfig.Subnet.String()))
			Expect(config["kawasaki.iptable-chain"]).To(Equal(networkConfig.IPTableChain))
			Expect(config["kawasaki.mtu"]).To(Equal(strconv.Itoa(networkConfig.Mtu)))
		})

		Context("when the configuration can't be created", func() {
			It("returns a wrapped error", func() {
				fakeConfigCreator.CreateReturns(kawasaki.NetworkConfig{}, errors.New("bad config"))
				_, err := networker.Network(logger, "some-handle", "1.2.3.4/30")
				Expect(err).To(MatchError("create network config: bad config"))
			})

			It("does not create a namespace", func() {
				fakeConfigCreator.CreateReturns(kawasaki.NetworkConfig{}, errors.New("bad config"))
				networker.Network(logger, "some-handle", "1.2.3.4/30")

				Expect(fakeNetnsMgr.CreateCallCount()).To(Equal(0))
			})
		})

		It("should create a network namespace", func() {
			networker.Network(logger, "some-handle", "")
			Expect(fakeNetnsMgr.CreateCallCount()).To(Equal(1))

			_, handle := fakeNetnsMgr.CreateArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		Context("when creating the network namespace fails", func() {
			BeforeEach(func() {
				fakeNetnsMgr.CreateReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				_, err := networker.Network(logger, "", "")
				Expect(err).To(MatchError("banana"))
			})

			It("should not configure the network", func() {
				_, err := networker.Network(logger, "", "")
				Expect(err).To(HaveOccurred())

				Expect(fakeConfigurer.ApplyCallCount()).To(Equal(0))
			})
		})

		It("should return the looked up network path", func() {
			fakeNetnsMgr.LookupReturns("/i/lost/my/banana", nil)

			path, err := networker.Network(logger, "", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal("/i/lost/my/banana"))
		})

		Context("when looking up the network namespace path fails", func() {
			BeforeEach(func() {
				fakeNetnsMgr.LookupReturns("", errors.New("banana"))
			})

			It("should return an error", func() {
				_, err := networker.Network(logger, "", "")
				Expect(err).To(MatchError("banana"))
			})

			It("should not configure the network", func() {
				_, err := networker.Network(logger, "", "")
				Expect(err).To(HaveOccurred())

				Expect(fakeConfigurer.ApplyCallCount()).To(Equal(0))
			})
		})

		It("should apply the configuration", func() {
			fakeNetnsMgr.LookupReturns("/i/lost/my/banana", nil)

			_, err := networker.Network(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConfigurer.ApplyCallCount()).To(Equal(1))
			_, actualCfg, path := fakeConfigurer.ApplyArgsForCall(0)
			Expect(path).To(Equal("/i/lost/my/banana"))
			Expect(actualCfg).To(Equal(networkConfig))
		})

		Context("when applying the configuration fails", func() {
			BeforeEach(func() {
				fakeConfigurer.ApplyReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				_, err := networker.Network(logger, "", "")
				Expect(err).To(MatchError("banana"))
			})

			It("destroys the network namespace", func() {
				_, err := networker.Network(logger, "banana-handle", "")
				Expect(err).To(HaveOccurred())

				Expect(fakeNetnsMgr.DestroyCallCount()).To(Equal(1))
				_, handle := fakeNetnsMgr.DestroyArgsForCall(0)
				Expect(handle).To(Equal("banana-handle"))
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
		Context("when the handle does not exist", func() {
			BeforeEach(func() {
				fakeConfigStore.GetReturns("", errors.New("Handle does not exist"))
			})

			It("should return the error", func() {
				Expect(networker.Destroy(logger, "non-existing-handle")).To(MatchError("Handle does not exist"))
			})

			It("should not destroy the network namespace", func() {
				networker.Destroy(logger, "non-existing-handle")

				Expect(fakeNetnsMgr.DestroyCallCount()).To(Equal(0))
			})
		})

		It("should destroy the network namespace", func() {
			networker.Destroy(logger, "some-handle")
			Expect(fakeNetnsMgr.DestroyCallCount()).To(Equal(1))

			_, handle := fakeNetnsMgr.DestroyArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		Context("when network namespace manager fails to destroy", func() {
			BeforeEach(func() {
				fakeNetnsMgr.DestroyReturns(errors.New("namespace deletion failed"))
			})

			It("should return the error", func() {
				err := networker.Destroy(logger, "some-handle")
				Expect(err).To(MatchError("namespace deletion failed"))
			})

			It("should not destroy the configuration", func() {
				Expect(networker.Destroy(logger, "some-handle")).NotTo(Succeed())

				Expect(fakeConfigurer.DestroyCallCount()).To(Equal(0))
			})
		})

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

package kawasaki_test

import (
	"errors"
	"fmt"
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets/fake_subnet_pool"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("ConfigStore", func() {
	var store kawasaki.ConfigStore

	BeforeEach(func() {
		store = &kawasaki.ConfigMap{}
	})

	Context("when a configuration is put", func() {
		var cfg kawasaki.NetworkConfig

		BeforeEach(func() {
			cfg = kawasaki.NetworkConfig{
				HostIntf:    "banana",
				Mtu:         1220,
				ContainerIP: net.ParseIP("127.82.34.12"),
			}

			store.Put("test", cfg)
		})

		It("should get back the configuration", func() {
			v, err := store.Get("test")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal(cfg))
		})

		Context("when another put uses an existing handle", func() {
			It("should overwrite the previous configuration", func() {
				newCfg := kawasaki.NetworkConfig{
					HostIntf:      "apple",
					Mtu:           1330,
					ContainerIntf: "banana",
				}

				store.Put("test", newCfg)

				v, err := store.Get("test")
				Expect(err).NotTo(HaveOccurred())

				Expect(v).To(Equal(newCfg))
			})
		})

		Context("and then removed", func() {
			It("should return error when we call get", func() {
				store.Remove("test")

				_, err := store.Get("test")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when a configuration is not put", func() {
		It("should return an error", func() {
			_, err := store.Get("spiderman")
			Expect(err).To(HaveOccurred())
		})
	})
})

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
			someIp, someSubnet, err := net.ParseCIDR("1.2.3.4/5")
			fakeSubnetPool.AcquireReturns(someSubnet, someIp, err)
			fakeConfig := kawasaki.NetworkConfig{
				HostIntf:   "spiderman-intf",
				BridgeName: "spiderman-bridge",
			}
			fakeConfigCreator.CreateReturns(fakeConfig, nil)

			networker.Network(logger, "some-handle", "1.2.3.4/30")
			Expect(fakeConfigStore.PutCallCount()).To(Equal(1))
			handle, cfg := fakeConfigStore.PutArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(cfg).To(Equal(fakeConfig))
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

			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana-iface",
			}

			fakeConfigCreator.CreateReturns(cfg, nil)

			_, err := networker.Network(logger, "some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConfigurer.ApplyCallCount()).To(Equal(1))
			_, actualCfg, path := fakeConfigurer.ApplyArgsForCall(0)
			Expect(path).To(Equal("/i/lost/my/banana"))
			Expect(actualCfg).To(Equal(cfg))
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
				fakeConfigStore.GetReturns(kawasaki.NetworkConfig{}, errors.New("Handle does not exist"))
			})

			It("should return the error", func() {
				Expect(networker.Destroy(logger, "non-existing-handle")).To(MatchError("Handle does not exist"))
			})

			It("should not destroy the network namespace", func() {
				networker.Destroy(logger, "non-existing-handle")

				Expect(fakeNetnsMgr.DestroyCallCount()).To(Equal(0))
			})
		})

		It("should remove the handle config pair in ConfigStore", func() {
			Expect(networker.Destroy(logger, "spiderman-handle")).To(Succeed())
			Expect(fakeConfigStore.RemoveCallCount()).To(Equal(1))
			Expect(fakeConfigStore.RemoveArgsForCall(0)).To(Equal("spiderman-handle"))
		})

		It("should destroy the network namespace", func() {
			networker.Network(logger, "some-handle", "1.2.3.4/30")

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
				err := networker.Destroy(logger, "error-handle")
				Expect(err).To(MatchError("namespace deletion failed"))
			})

			It("should not destroy the configuration", func() {
				Expect(networker.Destroy(logger, "error-handle")).NotTo(Succeed())

				Expect(fakeConfigurer.DestroyCallCount()).To(Equal(0))
			})
		})

		It("should destroy the configuration", func() {
			fakeConfig := kawasaki.NetworkConfig{
				HostIntf:   "spiderman-intf",
				BridgeName: "spiderman-bridge",
			}
			fakeConfigStore.GetReturns(fakeConfig, nil)

			Expect(networker.Destroy(logger, "some-handle")).To(Succeed())

			Expect(fakeConfigStore.GetCallCount()).To(Equal(1))
			Expect(fakeConfigStore.GetArgsForCall(0)).To(Equal("some-handle"))

			Expect(fakeConfigurer.DestroyCallCount()).To(Equal(1))
			_, netConfig := fakeConfigurer.DestroyArgsForCall(0)
			Expect(netConfig).To(Equal(fakeConfig))
		})

		Context("when the configuration is not destroyed", func() {
			It("should return the error", func() {
				fakeConfig := kawasaki.NetworkConfig{
					HostIntf:   "spiderman-intf",
					BridgeName: "spiderman-bridge",
				}
				fakeConfigStore.GetReturns(fakeConfig, nil)

				fakeConfigurer.DestroyReturns(errors.New("spiderman-error"))

				err := networker.Destroy(logger, "some-handle")
				Expect(err).To(MatchError("spiderman-error"))
			})
		})

		Describe("NetIt", func() {
			const (
				externalPort  uint32 = 123
				containerPort uint32 = 456
				handle        string = "handle"
			)

			var networkConfig kawasaki.NetworkConfig

			BeforeEach(func() {
				networkConfig = kawasaki.NetworkConfig{BridgeName: "fake-bridge-name"}

				fakeConfigStore.GetReturns(networkConfig, nil)
				_, err := networker.Network(logger, handle, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("calls the PortForwarder with correct parameters", func() {
				networker.NetIn(handle, externalPort, containerPort)
				Expect(fakePortForwarder.ForwardCallCount()).To(Equal(1))

				actualSpec := fakePortForwarder.ForwardArgsForCall(0)
				Expect(actualSpec.NetworkConfig.BridgeName).To(Equal("fake-bridge-name"))
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
					fakeConfigStore.GetReturns(kawasaki.NetworkConfig{}, errors.New("Handle does not exist"))
				})
				It("returns an error", func() {
					_, _, err := networker.NetIn("nonexistent", 0, 0)
					Expect(err).To(MatchError("Handle does not exist"))
				})
			})
		})
	})
})

package kawasaki_test

import (
	"errors"
	"io/ioutil"
	"net"
	"os"

	"code.cloudfoundry.org/guardian/kawasaki"
	fakes "code.cloudfoundry.org/guardian/kawasaki/kawasakifakes"
	"code.cloudfoundry.org/guardian/kawasaki/netns"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurer", func() {
	var (
		fakeDnsResolvConfigurer  *fakes.FakeDnsResolvConfigurer
		fakeHostConfigurer       *fakes.FakeHostConfigurer
		fakeContainerConfigurer  *fakes.FakeContainerConfigurer
		fakeInstanceChainCreator *fakes.FakeInstanceChainCreator

		dummyFileOpener netns.Opener

		netnsFD *os.File

		configurer kawasaki.Configurer

		logger lager.Logger
	)

	BeforeEach(func() {
		fakeDnsResolvConfigurer = new(fakes.FakeDnsResolvConfigurer)

		fakeHostConfigurer = new(fakes.FakeHostConfigurer)
		fakeContainerConfigurer = new(fakes.FakeContainerConfigurer)
		fakeInstanceChainCreator = new(fakes.FakeInstanceChainCreator)

		var err error
		netnsFD, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		dummyFileOpener = func(path string) (*os.File, error) {
			return netnsFD, nil
		}

		configurer = kawasaki.NewConfigurer(fakeDnsResolvConfigurer, fakeHostConfigurer, fakeContainerConfigurer, fakeInstanceChainCreator)

		logger = lagertest.NewTestLogger("test")
	})

	AfterEach(func() {
		Expect(os.Remove(netnsFD.Name())).To(Succeed())
	})

	Describe("Apply", func() {
		It("configures dns", func() {
			Expect(configurer.Apply(logger, kawasaki.NetworkConfig{ContainerHandle: "h"}, 42)).To(Succeed())
			Expect(fakeDnsResolvConfigurer.ConfigureCallCount()).To(Equal(1))
			_, cfg, pid := fakeDnsResolvConfigurer.ConfigureArgsForCall(0)
			Expect(cfg).To(Equal(kawasaki.NetworkConfig{ContainerHandle: "h"}))
			Expect(pid).To(Equal(42))
		})

		Context("when resolv configuration fails", func() {
			It("returns the error", func() {
				fakeDnsResolvConfigurer.ConfigureReturns(errors.New("baboom"))
				Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, 42)).To(MatchError("baboom"))
			})
		})

		It("applies the configuration in the host", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}

			Expect(configurer.Apply(logger, cfg, 42)).To(Succeed())

			Expect(fakeHostConfigurer.ApplyCallCount()).To(Equal(1))
			_, appliedCfg, pid := fakeHostConfigurer.ApplyArgsForCall(0)
			Expect(appliedCfg).To(Equal(cfg))
			Expect(pid).To(Equal(42))
		})

		Context("if applying the host config fails", func() {
			BeforeEach(func() {
				fakeHostConfigurer.ApplyReturns(errors.New("boom"))
			})

			It("returns the error", func() {
				Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, 42)).To(MatchError("boom"))
			})

			It("does not configure the container", func() {
				Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, 42)).To(MatchError("boom"))
				Expect(fakeContainerConfigurer.ApplyCallCount()).To(Equal(0))
			})

			It("does not configure IPTables", func() {
				Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, 42)).To(MatchError("boom"))
				Expect(fakeInstanceChainCreator.CreateCallCount()).To(Equal(0))
			})
		})

		It("applies the iptable configuration", func() {
			_, subnet, _ := net.ParseCIDR("1.2.3.4/5")
			cfg := kawasaki.NetworkConfig{
				IPTablePrefix:   "the-iptable",
				IPTableInstance: "instance",
				BridgeName:      "the-bridge-name",
				ContainerIP:     net.ParseIP("1.2.3.4"),
				ContainerHandle: "some-handle",
				Subnet:          subnet,
			}

			Expect(configurer.Apply(logger, cfg, 42)).To(Succeed())
			Expect(fakeInstanceChainCreator.CreateCallCount()).To(Equal(1))
			_, handle, instanceChain, bridgeName, ip, subnet := fakeInstanceChainCreator.CreateArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(instanceChain).To(Equal("instance"))
			Expect(bridgeName).To(Equal("the-bridge-name"))
			Expect(ip).To(Equal(net.ParseIP("1.2.3.4")))
			Expect(subnet).To(Equal(subnet))
		})

		Context("when applying IPTables configuration fails", func() {
			It("returns the error", func() {
				fakeInstanceChainCreator.CreateReturns(errors.New("oh no"))
				Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, 42)).To(MatchError("oh no"))
			})
		})

		It("applies the configuration in the container", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}

			Expect(configurer.Apply(logger, cfg, 42)).To(Succeed())

			Expect(fakeContainerConfigurer.ApplyCallCount()).To(Equal(1))
			_, cfgArg, pid := fakeContainerConfigurer.ApplyArgsForCall(0)
			Expect(cfgArg).To(Equal(cfg))
			Expect(pid).To(Equal(42))
		})

		Context("if container configuration fails", func() {
			BeforeEach(func() {
				fakeContainerConfigurer.ApplyReturns(errors.New("banana"))
			})

			It("returns the error", func() {
				Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, 42)).To(MatchError("banana"))
			})
		})
	})

	Describe("DestroyBridge", func() {
		It("should destroy the host configuration", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}
			Expect(configurer.DestroyBridge(logger, cfg)).To(Succeed())

			Expect(fakeHostConfigurer.DestroyCallCount()).To(Equal(1))
			Expect(fakeHostConfigurer.DestroyArgsForCall(0)).To(Equal(cfg))
		})

		Context("when it fails to destroy the host configuration", func() {
			It("should return the error", func() {
				fakeHostConfigurer.DestroyReturns(errors.New("spiderman-error"))

				err := configurer.DestroyBridge(logger, kawasaki.NetworkConfig{})
				Expect(err).To(MatchError(ContainSubstring("spiderman-error")))
			})
		})
	})

	Describe("DestroyIPTablesRules", func() {
		It("should tear down the IP tables chains", func() {
			cfg := kawasaki.NetworkConfig{
				IPTablePrefix:   "chain-of-",
				IPTableInstance: "sausages",
			}
			Expect(configurer.DestroyIPTablesRules(logger, cfg)).To(Succeed())

			Expect(fakeInstanceChainCreator.DestroyCallCount()).To(Equal(1))
			_, instance := fakeInstanceChainCreator.DestroyArgsForCall(0)
			Expect(instance).To(Equal("sausages"))
		})

		Context("when the teardown of ip tables fail", func() {
			BeforeEach(func() {
				fakeInstanceChainCreator.DestroyReturns(errors.New("ananas is the best"))
			})

			It("should return the error", func() {
				cfg := kawasaki.NetworkConfig{}
				Expect(configurer.DestroyIPTablesRules(logger, cfg)).To(MatchError(ContainSubstring("ananas is the best")))
			})
		})
	})
})

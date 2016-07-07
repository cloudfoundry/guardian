package kawasaki_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	fakes "github.com/cloudfoundry-incubator/guardian/kawasaki/kawasakifakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurer", func() {
	var (
		fakeResolvConfFactory      *fakes.FakeDnsResolvConfFactory
		fakeDnsResolvConfigurer    *fakes.FakeDnsResolvConfigurer
		fakeHostConfigurer         *fakes.FakeHostConfigurer
		fakeContainerConfigApplier *fakes.FakeContainerApplier
		fakeInstanceChainCreator   *fakes.FakeInstanceChainCreator
		fakeNsExecer               *fakes.FakeNetnsExecer

		netnsFD *os.File

		configurer kawasaki.Configurer

		logger lager.Logger
	)

	BeforeEach(func() {
		fakeResolvConfFactory = new(fakes.FakeDnsResolvConfFactory)
		fakeDnsResolvConfigurer = new(fakes.FakeDnsResolvConfigurer)
		fakeResolvConfFactory.CreateDNSResolvConfigurerReturns(fakeDnsResolvConfigurer)

		fakeHostConfigurer = new(fakes.FakeHostConfigurer)
		fakeContainerConfigApplier = new(fakes.FakeContainerApplier)
		fakeInstanceChainCreator = new(fakes.FakeInstanceChainCreator)

		fakeNsExecer = new(fakes.FakeNetnsExecer)

		var err error
		netnsFD, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		configurer = kawasaki.NewConfigurer(fakeResolvConfFactory, fakeHostConfigurer, fakeContainerConfigApplier, fakeInstanceChainCreator, fakeNsExecer)

		logger = lagertest.NewTestLogger("test")
	})

	AfterEach(func() {
		Expect(os.Remove(netnsFD.Name())).To(Succeed())
	})

	Describe("Apply", func() {
		It("configures dns", func() {
			Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(Succeed())
			Expect(fakeDnsResolvConfigurer.ConfigureCallCount()).To(Equal(1))
		})

		Context("when dns configuration fails", func() {
			It("returns the error", func() {
				fakeDnsResolvConfigurer.ConfigureReturns(errors.New("baboom"))
				Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(MatchError("baboom"))
			})
		})

		Context("when the ns path can be opened", func() {
			It("closes the file descriptor of the ns path", func() {
				cfg := kawasaki.NetworkConfig{
					ContainerIntf: "banana",
				}

				Expect(configurer.Apply(logger, cfg, netnsFD.Name(), "bundle-path")).To(Succeed())
				command := fmt.Sprintf("lsof %s | wc -l", netnsFD.Name())
				output, err := exec.Command("sh", "-c", command).Output()
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(string(output))).To(Equal("2"))
			})

			It("applies the configuration in the host", func() {
				cfg := kawasaki.NetworkConfig{
					ContainerIntf: "banana",
				}

				Expect(configurer.Apply(logger, cfg, netnsFD.Name(), "bundle-path")).To(Succeed())

				Expect(fakeHostConfigurer.ApplyCallCount()).To(Equal(1))
				_, appliedCfg, fd := fakeHostConfigurer.ApplyArgsForCall(0)
				Expect(appliedCfg).To(Equal(cfg))
				Expect(fd.Name()).To(Equal(netnsFD.Name()))
			})

			Context("if applying the host config fails", func() {
				BeforeEach(func() {
					fakeHostConfigurer.ApplyReturns(errors.New("boom"))
				})

				It("returns the error", func() {
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(MatchError("boom"))
				})

				It("does not configure the container", func() {
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(MatchError("boom"))
					Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(0))
				})

				It("does not configure IPTables", func() {
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(MatchError("boom"))
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

				Expect(configurer.Apply(logger, cfg, netnsFD.Name(), "bundle-path")).To(Succeed())
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
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(MatchError("oh no"))
				})
			})

			It("calls the namespace execer and applies the configuration in the container", func() {
				cfg := kawasaki.NetworkConfig{
					ContainerIntf: "banana",
				}

				Expect(configurer.Apply(logger, cfg, netnsFD.Name(), "bundle-path")).To(Succeed())

				Expect(fakeNsExecer.ExecCallCount()).To(Equal(1))
				fd, cb := fakeNsExecer.ExecArgsForCall(0)
				Expect(fd.Name()).To(Equal(netnsFD.Name()))

				Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(0))
				cb()
				Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(1))

				_, cfgArg := fakeContainerConfigApplier.ApplyArgsForCall(0)
				Expect(cfgArg).To(Equal(cfg))
			})

			Context("if entering the namespace fails", func() {
				It("returns the error", func() {
					fakeNsExecer.ExecReturns(errors.New("boom"))
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(MatchError("boom"))
				})
			})

			Context("if container configuration fails", func() {
				It("returns the error", func() {
					fakeNsExecer.ExecStub = func(_ *os.File, cb func() error) error {
						return cb()
					}

					fakeContainerConfigApplier.ApplyReturns(errors.New("banana"))
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name(), "bundle-path")).To(MatchError("banana"))
				})
			})
		})

		Context("when the ns path cannot be opened", func() {
			It("returns an error", func() {
				err := configurer.Apply(logger, kawasaki.NetworkConfig{}, "DOESNOTEXIST", "bundle-path")
				Expect(err).To(HaveOccurred())
			})

			It("does not configure anything", func() {
				configurer.Apply(logger, kawasaki.NetworkConfig{}, "DOESNOTEXIST", "bundle-path")
				Expect(fakeHostConfigurer.ApplyCallCount()).To(Equal(0))
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

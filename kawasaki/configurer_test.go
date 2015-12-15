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
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurer", func() {
	var (
		fakeHostConfigurer         *fakes.FakeHostConfigurer
		fakeContainerConfigApplier *fakes.FakeContainerApplier
		fakeIPTablesConfigurer     *fakes.FakeIPTablesConfigurer
		fakeNsExecer               *fakes.FakeNetnsExecer

		netnsFD *os.File

		configurer kawasaki.Configurer

		logger lager.Logger
	)

	BeforeEach(func() {
		fakeHostConfigurer = new(fakes.FakeHostConfigurer)
		fakeContainerConfigApplier = new(fakes.FakeContainerApplier)
		fakeIPTablesConfigurer = new(fakes.FakeIPTablesConfigurer)

		fakeNsExecer = new(fakes.FakeNetnsExecer)

		var err error
		netnsFD, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())
		configurer = kawasaki.NewConfigurer(fakeHostConfigurer, fakeContainerConfigApplier, fakeIPTablesConfigurer, fakeNsExecer)

		logger = lagertest.NewTestLogger("test")
	})

	AfterEach(func() {
		Expect(os.Remove(netnsFD.Name())).To(Succeed())
	})

	Describe("Apply", func() {
		Context("when the ns path can be opened", func() {
			It("closes the file descriptor of the ns path", func() {
				cfg := kawasaki.NetworkConfig{
					ContainerIntf: "banana",
				}

				Expect(configurer.Apply(logger, cfg, netnsFD.Name())).To(Succeed())
				command := fmt.Sprintf("lsof %s | wc -l", netnsFD.Name())
				output, err := exec.Command("sh", "-c", command).CombinedOutput()
				Expect(err).NotTo(HaveOccurred())
				Expect(strings.TrimSpace(string(output))).To(Equal("2"))
			})

			It("applies the configuration in the host", func() {
				cfg := kawasaki.NetworkConfig{
					ContainerIntf: "banana",
				}

				Expect(configurer.Apply(logger, cfg, netnsFD.Name())).To(Succeed())

				Expect(fakeHostConfigurer.ApplyCallCount()).To(Equal(1))
				appliedCfg, fd := fakeHostConfigurer.ApplyArgsForCall(0)
				Expect(appliedCfg).To(Equal(cfg))
				Expect(fd.Name()).To(Equal(netnsFD.Name()))
			})

			Context("if applying the host config fails", func() {
				BeforeEach(func() {
					fakeHostConfigurer.ApplyReturns(errors.New("boom"))
				})

				It("returns the error", func() {
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
				})

				It("does not configure the container", func() {
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
					Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(0))
				})

				It("does not configure IPTables", func() {
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
					Expect(fakeIPTablesConfigurer.ApplyCallCount()).To(Equal(0))
				})
			})

			It("applies the iptable configuration", func() {
				_, subnet, _ := net.ParseCIDR("1.2.3.4/5")
				cfg := kawasaki.NetworkConfig{
					IPTableChain: "the-iptable-chain",
					BridgeName:   "the-bridge-name",
					ContainerIP:  net.ParseIP("1.2.3.4"),
					Subnet:       subnet,
				}

				Expect(configurer.Apply(logger, cfg, netnsFD.Name())).To(Succeed())
				Expect(fakeIPTablesConfigurer.ApplyCallCount()).To(Equal(1))
				instanceChain, bridgeName, ip, subnet := fakeIPTablesConfigurer.ApplyArgsForCall(0)
				Expect(instanceChain).To(Equal("the-iptable-chain"))
				Expect(bridgeName).To(Equal("the-bridge-name"))
				Expect(ip).To(Equal(net.ParseIP("1.2.3.4")))
				Expect(subnet).To(Equal(subnet))
			})

			Context("when applying IPTables configuration fails", func() {
				It("returns the error", func() {
					fakeIPTablesConfigurer.ApplyReturns(errors.New("oh no"))
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("oh no"))
				})
			})

			It("calls the namespace execer and applies the configuration in the container", func() {
				cfg := kawasaki.NetworkConfig{
					ContainerIntf: "banana",
				}

				Expect(configurer.Apply(logger, cfg, netnsFD.Name())).To(Succeed())

				Expect(fakeNsExecer.ExecCallCount()).To(Equal(1))
				fd, cb := fakeNsExecer.ExecArgsForCall(0)
				Expect(fd.Name()).To(Equal(netnsFD.Name()))

				Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(0))
				cb()
				Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(1))
				Expect(fakeContainerConfigApplier.ApplyArgsForCall(0)).To(Equal(cfg))
			})

			Context("if entering the namespace fails", func() {
				It("returns the error", func() {
					fakeNsExecer.ExecReturns(errors.New("boom"))
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
				})
			})

			Context("if container configuration fails", func() {
				It("returns the error", func() {
					fakeNsExecer.ExecStub = func(_ *os.File, cb func() error) error {
						return cb()
					}

					fakeContainerConfigApplier.ApplyReturns(errors.New("banana"))
					Expect(configurer.Apply(logger, kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("banana"))
				})
			})
		})

		Context("when the ns path cannot be opened", func() {
			It("returns an error", func() {
				err := configurer.Apply(logger, kawasaki.NetworkConfig{}, "DOESNOTEXIST")
				Expect(err).To(HaveOccurred())
			})

			It("does not configure anything", func() {
				configurer.Apply(logger, kawasaki.NetworkConfig{}, "DOESNOTEXIST")
				Expect(fakeHostConfigurer.ApplyCallCount()).To(Equal(0))
			})
		})

	})

	Describe("Destroy", func() {
		It("should tear down the IP tables chains", func() {
			cfg := kawasaki.NetworkConfig{
				IPTableChain: "chain-of-sausages",
			}
			Expect(configurer.Destroy(logger, cfg)).To(Succeed())

			Expect(fakeIPTablesConfigurer.DestroyCallCount()).To(Equal(1))
			Expect(fakeIPTablesConfigurer.DestroyArgsForCall(0)).To(Equal("chain-of-sausages"))
		})

		Context("when the teardown of ip tables fail", func() {
			BeforeEach(func() {
				fakeIPTablesConfigurer.DestroyReturns(errors.New("ananas is the best"))
			})

			It("should return the error", func() {
				cfg := kawasaki.NetworkConfig{}
				Expect(configurer.Destroy(logger, cfg)).To(MatchError(ContainSubstring("ananas is the best")))
			})

			It("should not destroy the host configuration", func() {
				cfg := kawasaki.NetworkConfig{}
				Expect(configurer.Destroy(logger, cfg)).NotTo(Succeed())

				Expect(fakeHostConfigurer.DestroyCallCount()).To(Equal(0))
			})
		})

		It("should destroy the host configuration", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}
			Expect(configurer.Destroy(logger, cfg)).To(Succeed())

			Expect(fakeHostConfigurer.DestroyCallCount()).To(Equal(1))
			Expect(fakeHostConfigurer.DestroyArgsForCall(0)).To(Equal(cfg))
		})

		Context("when it fails to destroy the host configuration", func() {
			It("should return the error", func() {
				fakeHostConfigurer.DestroyReturns(errors.New("spiderman-error"))

				err := configurer.Destroy(logger, kawasaki.NetworkConfig{})
				Expect(err).To(MatchError(ContainSubstring("spiderman-error")))
			})
		})
	})
})

package kawasaki_test

import (
	"errors"
	"io/ioutil"
	"net"
	"os"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConfigApplier", func() {
	var (
		fakeHostConfigApplier      *fakes.FakeHostApplier
		fakeContainerConfigApplier *fakes.FakeContainerApplier
		fakeIPTablesApplier        *fakes.FakeIPTablesApplier
		fakeNsExecer               *fakes.FakeNetnsExecer

		netnsFD *os.File

		applier kawasaki.ConfigApplier
	)

	BeforeEach(func() {
		fakeHostConfigApplier = new(fakes.FakeHostApplier)
		fakeContainerConfigApplier = new(fakes.FakeContainerApplier)
		fakeIPTablesApplier = new(fakes.FakeIPTablesApplier)

		fakeNsExecer = new(fakes.FakeNetnsExecer)

		var err error
		netnsFD, err = ioutil.TempFile("", "")
		Expect(err).NotTo(HaveOccurred())

		applier = kawasaki.NewConfigApplier(fakeHostConfigApplier, fakeContainerConfigApplier, fakeIPTablesApplier, fakeNsExecer)
	})

	AfterEach(func() {
		Expect(os.Remove(netnsFD.Name())).To(Succeed())
	})

	Describe("Apply", func() {
		Context("when the ns path cannot be opened", func() {
			It("returns an error", func() {
				err := applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, "DOESNOTEXIST")
				Expect(err).To(HaveOccurred())
			})

			It("does not configure anything", func() {
				applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, "DOESNOTEXIST")
				Expect(fakeHostConfigApplier.ApplyCallCount()).To(Equal(0))
			})
		})

		It("applies the configuration in the host", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}

			Expect(applier.Apply(lagertest.NewTestLogger("test"), cfg, netnsFD.Name())).To(Succeed())

			Expect(fakeHostConfigApplier.ApplyCallCount()).To(Equal(1))
			appliedCfg, fd := fakeHostConfigApplier.ApplyArgsForCall(0)
			Expect(appliedCfg).To(Equal(cfg))
			Expect(fd.Name()).To(Equal(netnsFD.Name()))
		})

		Context("if applying the host config fails", func() {
			BeforeEach(func() {
				fakeHostConfigApplier.ApplyReturns(errors.New("boom"))
			})

			It("returns the error", func() {
				Expect(applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
			})

			It("does not configure the container", func() {
				Expect(applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
				Expect(fakeContainerConfigApplier.ApplyCallCount()).To(Equal(0))
			})

			It("does not configure IPTables", func() {
				Expect(applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
				Expect(fakeIPTablesApplier.ApplyCallCount()).To(Equal(0))
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

			Expect(applier.Apply(lagertest.NewTestLogger("test"), cfg, netnsFD.Name())).To(Succeed())
			Expect(fakeIPTablesApplier.ApplyCallCount()).To(Equal(1))
			instanceChain, bridgeName, ip, subnet := fakeIPTablesApplier.ApplyArgsForCall(0)
			Expect(instanceChain).To(Equal("the-iptable-chain"))
			Expect(bridgeName).To(Equal("the-bridge-name"))
			Expect(ip).To(Equal(net.ParseIP("1.2.3.4")))
			Expect(subnet).To(Equal(subnet))
		})

		Context("when applying IPTables configuration fails", func() {
			It("returns the error", func() {
				fakeIPTablesApplier.ApplyReturns(errors.New("oh no"))
				Expect(applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("oh no"))
			})
		})

		It("calls the namespace execer and applies the configuration in the container", func() {
			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana",
			}

			Expect(applier.Apply(lagertest.NewTestLogger("test"), cfg, netnsFD.Name())).To(Succeed())

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
				Expect(applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("boom"))
			})
		})

		Context("if container configuration fails", func() {
			It("returns the error", func() {
				fakeNsExecer.ExecStub = func(_ *os.File, cb func() error) error {
					return cb()
				}

				fakeContainerConfigApplier.ApplyReturns(errors.New("banana"))
				Expect(applier.Apply(lagertest.NewTestLogger("test"), kawasaki.NetworkConfig{}, netnsFD.Name())).To(MatchError("banana"))
			})
		})
	})
})

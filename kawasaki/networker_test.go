package kawasaki_test

import (
	"errors"
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/subnets/fake_subnet_pool"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Networker", func() {
	var (
		fakeNetnsMgr      *fakes.FakeNetnsMgr
		fakeSpecParser    *fakes.FakeSpecParser
		fakeSubnetPool    *fake_subnet_pool.FakePool
		fakeConfigCreator *fakes.FakeConfigCreator
		fakeConfigApplier *fakes.FakeConfigApplier
		networker         *kawasaki.Networker
	)

	BeforeEach(func() {
		fakeNetnsMgr = new(fakes.FakeNetnsMgr)
		fakeSpecParser = new(fakes.FakeSpecParser)
		fakeSubnetPool = new(fake_subnet_pool.FakePool)
		fakeConfigApplier = new(fakes.FakeConfigApplier)
		fakeConfigCreator = new(fakes.FakeConfigCreator)

		networker = kawasaki.New(
			fakeNetnsMgr,
			fakeSpecParser,
			fakeSubnetPool,
			fakeConfigCreator,
			fakeConfigApplier,
		)
	})

	Describe("Network", func() {
		It("parses the spec", func() {
			networker.Network("some-handle", "1.2.3.4/30")
			Expect(fakeSpecParser.ParseCallCount()).To(Equal(1))
			Expect(fakeSpecParser.ParseArgsForCall(0)).To(Equal("1.2.3.4/30"))
		})

		It("returns an error if the spec can't be parsed", func() {
			fakeSpecParser.ParseReturns(nil, nil, errors.New("no parsey"))
			_, err := networker.Network("some-handle", "1.2.3.4/30")
			Expect(err).To(MatchError("no parsey"))
		})

		It("acquires a subnet and IP", func() {
			someSubnetRequest := subnets.DynamicSubnetSelector
			someIpRequest := subnets.DynamicIPSelector
			fakeSpecParser.ParseReturns(someSubnetRequest, someIpRequest, nil)

			networker.Network("some-handle", "1.2.3.4/30")
			Expect(fakeSubnetPool.AcquireCallCount()).To(Equal(1))
			sr, ir := fakeSubnetPool.AcquireArgsForCall(0)
			Expect(sr).To(Equal(someSubnetRequest))
			Expect(ir).To(Equal(someIpRequest))
		})

		It("creates a network config", func() {
			someIp, someSubnet, err := net.ParseCIDR("1.2.3.4/5")
			fakeSubnetPool.AcquireReturns(someSubnet, someIp, err)

			networker.Network("some-handle", "1.2.3.4/30")
			Expect(fakeConfigCreator.CreateCallCount()).To(Equal(1))
			handle, subnet, ip := fakeConfigCreator.CreateArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(subnet).To(Equal(someSubnet))
			Expect(ip).To(Equal(someIp))
		})

		Context("when the configuration can't be created", func() {
			It("returns a wrapped error", func() {
				fakeConfigCreator.CreateReturns(kawasaki.NetworkConfig{}, errors.New("bad config"))
				_, err := networker.Network("some-handle", "1.2.3.4/30")
				Expect(err).To(MatchError("create network config: bad config"))
			})

			It("does not create a namespace", func() {
				fakeConfigCreator.CreateReturns(kawasaki.NetworkConfig{}, errors.New("bad config"))
				networker.Network("some-handle", "1.2.3.4/30")

				Expect(fakeNetnsMgr.CreateCallCount()).To(Equal(0))
			})
		})

		It("should create a network namespace", func() {
			networker.Network("some-handle", "")
			Expect(fakeNetnsMgr.CreateCallCount()).To(Equal(1))

			handle := fakeNetnsMgr.CreateArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
		})

		Context("when creating the network namespace fails", func() {
			BeforeEach(func() {
				fakeNetnsMgr.CreateReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				_, err := networker.Network("", "")
				Expect(err).To(MatchError("banana"))
			})

			It("should not configure the network", func() {
				_, err := networker.Network("", "")
				Expect(err).To(HaveOccurred())

				Expect(fakeConfigApplier.ApplyCallCount()).To(Equal(0))
			})
		})

		It("should return the looked up network path", func() {
			fakeNetnsMgr.LookupReturns("/i/lost/my/banana", nil)

			path, err := networker.Network("", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal("/i/lost/my/banana"))
		})

		Context("when looking up the network namespace path fails", func() {
			BeforeEach(func() {
				fakeNetnsMgr.LookupReturns("", errors.New("banana"))
			})

			It("should return an error", func() {
				_, err := networker.Network("", "")
				Expect(err).To(MatchError("banana"))
			})

			It("should not configure the network", func() {
				_, err := networker.Network("", "")
				Expect(err).To(HaveOccurred())

				Expect(fakeConfigApplier.ApplyCallCount()).To(Equal(0))
			})
		})

		It("should apply the configuration", func() {
			fakeNetnsMgr.LookupReturns("/i/lost/my/banana", nil)

			cfg := kawasaki.NetworkConfig{
				ContainerIntf: "banana-iface",
			}

			fakeConfigCreator.CreateReturns(cfg, nil)

			_, err := networker.Network("some-handle", "1.2.3.4/30")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeConfigApplier.ApplyCallCount()).To(Equal(1))
			actualCfg, path := fakeConfigApplier.ApplyArgsForCall(0)
			Expect(path).To(Equal("/i/lost/my/banana"))
			Expect(actualCfg).To(Equal(cfg))
		})

		Context("when applying the configuration fails", func() {
			BeforeEach(func() {
				fakeConfigApplier.ApplyReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				_, err := networker.Network("", "")
				Expect(err).To(MatchError("banana"))
			})

			It("destroys the network namespace", func() {
				_, err := networker.Network("banana-handle", "")
				Expect(err).To(HaveOccurred())

				Expect(fakeNetnsMgr.DestroyCallCount()).To(Equal(1))
				Expect(fakeNetnsMgr.DestroyArgsForCall(0)).To(Equal("banana-handle"))
			})
		})
	})
})

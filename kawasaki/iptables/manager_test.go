package iptables_test

import (
	"errors"
	"net"

	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/iptables/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var (
		fakeChains  []*fakes.FakeChain
		manager     *iptables.Manager
		containerID string
		bridgeName  string
		ip          net.IP
		network     *net.IPNet
	)

	BeforeEach(func() {
		fakeChains = []*fakes.FakeChain{}
		for i := 0; i < 2; i++ {
			fakeChain := new(fakes.FakeChain)
			fakeChains = append(fakeChains, fakeChain)
		}

		manager = &iptables.Manager{
			Chains: []iptables.Chain{fakeChains[0], fakeChains[1]},
		}
		containerID = "some-ctr-id"
		bridgeName = "some-bridge"
		var err error
		ip, network, err = net.ParseCIDR("1.2.3.4/28")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Apply", func() {
		It("should set up the chains", func() {
			Expect(manager.Apply(containerID, bridgeName, ip, network)).To(Succeed())
			for _, fakeChain := range fakeChains {
				Expect(fakeChain.SetupCallCount()).To(Equal(1))
				ctrID, br, i, n := fakeChain.SetupArgsForCall(0)
				Expect(ctrID).To(Equal(containerID))
				Expect(br).To(Equal(bridgeName))
				Expect(i).To(Equal(ip))
				Expect(n).To(Equal(network))
			}
		})

		It("should tear down the chains", func() {
			Expect(manager.Apply(containerID, bridgeName, ip, network)).To(Succeed())

			for _, fakeChain := range fakeChains {
				Expect(fakeChain.TeardownCallCount()).To(Equal(1))
				ctrID := fakeChain.TeardownArgsForCall(0)
				Expect(ctrID).To(Equal(containerID))
			}
		})

		Context("when tearing down a chain fails", func() {
			BeforeEach(func() {
				fakeChains[1].TeardownReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(manager.Apply(containerID, bridgeName, ip, network)).To(MatchError("banana"))
			})

			It("should not set up any chains", func() {
				for _, fakeChain := range fakeChains {
					Expect(fakeChain.SetupCallCount()).To(Equal(0))
				}
			})
		})

		Context("when setting up an early chain fails", func() {
			BeforeEach(func() {
				fakeChains[0].SetupReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(manager.Apply(containerID, bridgeName, ip, network)).To(MatchError("banana"))
			})

			It("should not setup subsequent chains", func() {
				Expect(manager.Apply(containerID, bridgeName, ip, network)).NotTo(Succeed())

				Expect(fakeChains[1].SetupCallCount()).To(Equal(0))
			})
		})

		Context("when setting up a late chain fails", func() {
			BeforeEach(func() {
				fakeChains[1].SetupReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(manager.Apply(containerID, bridgeName, ip, network)).To(MatchError("banana"))
			})

			It("should tear down the earlier chains", func() {
				Expect(manager.Apply(containerID, bridgeName, ip, network)).NotTo(Succeed())

				Expect(fakeChains[0].TeardownCallCount()).To(Equal(2))
				ctrID := fakeChains[0].TeardownArgsForCall(1)
				Expect(ctrID).To(Equal(containerID))

				Expect(fakeChains[1].TeardownCallCount()).To(Equal(1))

			})
		})
	})

	Describe("Destroy", func() {
		It("should tear down the chains", func() {
			Expect(manager.Destroy(containerID)).To(Succeed())

			for _, fakeChain := range fakeChains {
				Expect(fakeChain.TeardownCallCount()).To(Equal(1))
				ctrID := fakeChain.TeardownArgsForCall(0)
				Expect(ctrID).To(Equal(containerID))
			}
		})

		Context("when tearing down a chain fails", func() {
			BeforeEach(func() {
				fakeChains[0].TeardownReturns(errors.New("banana"))
			})

			It("should return an error", func() {
				Expect(manager.Destroy(containerID)).To(MatchError("banana"))
			})

			It("should tear down subsequent chains", func() {
				Expect(manager.Destroy(containerID)).NotTo(Succeed())

				Expect(fakeChains[1].TeardownCallCount()).To(Equal(1))
			})
		})
	})
})

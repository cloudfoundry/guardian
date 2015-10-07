package kawasaki_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Networker", func() {
	var (
		fakeNetnsMgr      *fakes.FakeNetnsMgr
		fakeConfigCreator *fakes.FakeConfigCreator
		fakeConfigApplier *fakes.FakeConfigApplier
		networker         *kawasaki.Networker
	)

	BeforeEach(func() {
		fakeNetnsMgr = new(fakes.FakeNetnsMgr)
		fakeConfigApplier = new(fakes.FakeConfigApplier)
		fakeConfigCreator = new(fakes.FakeConfigCreator)

		networker = kawasaki.New(
			fakeNetnsMgr,
			fakeConfigCreator,
			fakeConfigApplier,
		)
	})

	Describe("Network", func() {
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

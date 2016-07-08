package kawasaki_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	fakes "github.com/cloudfoundry-incubator/guardian/kawasaki/kawasakifakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CompositeNetworker", func() {
	var (
		compositeNetworker kawasaki.Networker
		fakeNetworkers     []*fakes.FakeNetworker
		containerSpec      garden.ContainerSpec
	)

	BeforeEach(func() {
		fakeNetworkers = []*fakes.FakeNetworker{
			new(fakes.FakeNetworker),
			new(fakes.FakeNetworker),
			new(fakes.FakeNetworker),
		}

		containerSpec = garden.ContainerSpec{
			Handle:  "handle",
			Network: "spec",
			Properties: garden.Properties{
				"some": "thing",
			},
		}
	})

	JustBeforeEach(func() {
		compositeNetworker = &kawasaki.CompositeNetworker{
			Networkers: []kawasaki.Networker{
				fakeNetworkers[0],
				fakeNetworkers[1],
				fakeNetworkers[2],
			},
		}
	})

	Describe("Capacity", func() {
		It("Returns the min of all networkers' capacities", func() {
			fakeNetworkers[0].CapacityReturns(3)
			fakeNetworkers[1].CapacityReturns(7)
			fakeNetworkers[2].CapacityReturns(5)
			Expect(compositeNetworker.Capacity()).To(BeNumerically("==", 3))
		})
	})

	Describe("NetIn", func() {
		It("delegates to the first networker", func() {
			fakeNetworkers[0].NetInReturns(1, 2, nil)

			hostPort, containerPort, err := compositeNetworker.NetIn(nil, "some-handle", 1, 2)
			Expect(err).NotTo(HaveOccurred())
			Expect(hostPort).To(BeEquivalentTo(1))
			Expect(containerPort).To(BeEquivalentTo(2))

			Expect(fakeNetworkers[0].NetInCallCount()).To(Equal(1))
			Expect(fakeNetworkers[1].NetInCallCount()).To(Equal(0))
			Expect(fakeNetworkers[2].NetInCallCount()).To(Equal(0))

			_, handle, p1, p2 := fakeNetworkers[0].NetInArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(p1).To(BeEquivalentTo(1))
			Expect(p2).To(BeEquivalentTo(2))
		})
	})

	Describe("NetOut", func() {
		It("delegates to the first networker", func() {
			err := compositeNetworker.NetOut(nil, "some-handle", garden.NetOutRule{})
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeNetworkers[0].NetOutCallCount()).To(Equal(1))
			Expect(fakeNetworkers[1].NetOutCallCount()).To(Equal(0))
			Expect(fakeNetworkers[2].NetOutCallCount()).To(Equal(0))

			_, handle, rule := fakeNetworkers[0].NetOutArgsForCall(0)
			Expect(handle).To(Equal("some-handle"))
			Expect(rule).To(Equal(garden.NetOutRule{}))
		})
	})

	Describe("Destroy", func() {
		shouldDelegateTo := func(fakeNetworker []*fakes.FakeNetworker) {
			for _, fakeNetworker := range fakeNetworkers {
				Expect(fakeNetworker.DestroyCallCount()).To(Equal(1))
				_, handle := fakeNetworker.DestroyArgsForCall(0)
				Expect(handle).To(Equal("some-handle"))
			}
		}

		It("should delegate to all networkers", func() {
			Expect(compositeNetworker.Destroy(nil, "some-handle")).To(Succeed())

			shouldDelegateTo(fakeNetworkers)
		})

		Context("when a networker fails", func() {
			It("returns the error", func() {
				fakeNetworkers[1].DestroyReturns(errors.New("haha"))

				Expect(compositeNetworker.Destroy(nil, "my-container")).To(MatchError("haha"))
			})
		})
	})

	Describe("Restore", func() {
		shouldDelegateTo := func(fakeNetworker []*fakes.FakeNetworker) {
			for _, fakeNetworker := range fakeNetworkers {
				Expect(fakeNetworker.RestoreCallCount()).To(Equal(1))
				_, handle := fakeNetworker.RestoreArgsForCall(0)
				Expect(handle).To(Equal("some-handle"))
			}
		}

		It("should delegate to all networkers", func() {
			Expect(compositeNetworker.Restore(nil, "some-handle")).To(Succeed())

			shouldDelegateTo(fakeNetworkers)
		})

		Context("when a networker fails", func() {
			It("returns the error", func() {
				fakeNetworkers[1].RestoreReturns(errors.New("haha"))

				Expect(compositeNetworker.Restore(nil, "my-container")).To(MatchError("haha"))
			})
		})
	})

	Describe("Destroy", func() {
		shouldDelegateTo := func(fakeNetworker []*fakes.FakeNetworker) {
			for _, fakeNetworker := range fakeNetworkers {
				Expect(fakeNetworker.DestroyCallCount()).To(Equal(1))
				_, handle := fakeNetworker.DestroyArgsForCall(0)
				Expect(handle).To(Equal("some-handle"))
			}
		}

		It("should delegate to all networkers", func() {
			Expect(compositeNetworker.Destroy(nil, "some-handle")).To(Succeed())

			shouldDelegateTo(fakeNetworkers)
		})

		Context("when a networker fails", func() {
			It("returns the error", func() {
				fakeNetworkers[1].DestroyReturns(errors.New("haha"))

				Expect(compositeNetworker.Destroy(nil, "my-container")).To(MatchError("haha"))
			})
		})
	})

	Describe("Network", func() {
		shouldDelegateTo := func(fakeNetworker []*fakes.FakeNetworker) {
			for _, fakeNetworker := range fakeNetworkers {
				Expect(fakeNetworker.NetworkCallCount()).To(Equal(1))
				_, spec, pid := fakeNetworker.NetworkArgsForCall(0)
				Expect(spec).To(Equal(containerSpec))
				Expect(pid).To(Equal(42))
			}
		}

		It("delegates to all netwokers", func() {
			err := compositeNetworker.Network(nil, containerSpec, 42)
			Expect(err).NotTo(HaveOccurred())
			shouldDelegateTo(fakeNetworkers)
		})

		Context("when a networker fails", func() {
			It("returns the error", func() {
				fakeNetworkers[1].NetworkReturns(errors.New("kaput"))
				Expect(compositeNetworker.Network(nil, containerSpec, 42)).To(MatchError("kaput"))
			})
		})
	})

})

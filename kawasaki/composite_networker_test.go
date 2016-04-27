package kawasaki_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CompositeNetworker", func() {
	var (
		compositeNetworker kawasaki.Networker
		fakeNetworker      *fakes.FakeNetworker
		extraHooks         []kawasaki.NetworkHooker
		containerSpec      garden.ContainerSpec
	)

	BeforeEach(func() {
		fakeNetworker = new(fakes.FakeNetworker)
		extraHooks = []kawasaki.NetworkHooker{}
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
			Networker:  fakeNetworker,
			ExtraHooks: extraHooks,
		}
	})

	It("delegates to the netwoker", func() {
		_, err := compositeNetworker.Hooks(nil, containerSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeNetworker.HooksCallCount()).To(Equal(1))

		_, spec := fakeNetworker.HooksArgsForCall(0)
		Expect(spec).To(Equal(containerSpec))
	})

	It("returns the hooks from the networker", func() {
		fakeNetworker.HooksReturns([]gardener.Hooks{
			gardener.Hooks{},
		}, nil)

		hooks, err := compositeNetworker.Hooks(nil, containerSpec)
		Expect(err).NotTo(HaveOccurred())
		Expect(hooks).To(Equal([]gardener.Hooks{
			gardener.Hooks{},
		}))
	})

	Context("when the networker fails", func() {
		It("returns the error", func() {
			fakeNetworker.HooksReturns(nil, errors.New("kaput-hook"))

			_, err := compositeNetworker.Hooks(nil, containerSpec)
			Expect(err).To(MatchError("kaput-hook"))
		})
	})

	Context("when extra hooks are supplied", func() {
		var hooker *fakes.FakeNetworkHooker

		BeforeEach(func() {
			hooker = new(fakes.FakeNetworkHooker)
			extraHooks = append(extraHooks, hooker)

			fakeNetworker.HooksReturns(
				[]gardener.Hooks{
					gardener.Hooks{
						Prestart: gardener.Hook{
							Path: "/some/prestaaaarthooook",
							Args: []string{"spiderman"},
						},
					},
				}, nil)

			hooker.HooksReturns(
				gardener.Hooks{
					Prestart: gardener.Hook{
						Path: "/some/prestarthook",
						Args: []string{"yo"},
					},
				}, nil)

		})

		It("delegates the correct arguments to the extra hooks", func() {
			_, err := compositeNetworker.Hooks(nil, containerSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(hooker.HooksCallCount()).To(Equal(1))
			_, spec := hooker.HooksArgsForCall(0)
			Expect(spec).To(Equal(containerSpec))
		})

		It("returns the netwokrs hooks followed by any extra hooks", func() {
			hooks, err := compositeNetworker.Hooks(nil, containerSpec)
			Expect(err).NotTo(HaveOccurred())

			Expect(hooks).To(ContainElement(gardener.Hooks{
				Prestart: gardener.Hook{
					Path: "/some/prestaaaarthooook",
					Args: []string{"spiderman"},
				},
			}))

			Expect(hooks).To(ContainElement(gardener.Hooks{
				Prestart: gardener.Hook{
					Path: "/some/prestarthook",
					Args: []string{"yo"},
				},
			}))
		})

		Context("when the hook returns an error", func() {
			It("returns the error", func() {
				hooker.HooksReturns(gardener.Hooks{}, errors.New("kaput-hook-2"))
				_, err := compositeNetworker.Hooks(nil, containerSpec)

				Expect(err).To(MatchError("kaput-hook-2"))
			})
		})
	})
})

package kawasaki_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/guardian/kawasaki"
	"github.com/cloudfoundry-incubator/guardian/kawasaki/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("HookActioner", func() {
	var (
		fakeConfigurer          *fakes.FakeConfigurer
		fakeDnsResolvConfigurer *fakes.FakeDnsResolvConfigurer
		hookActioner            *kawasaki.HookActioner
		log                     lager.Logger
	)

	BeforeEach(func() {
		fakeConfigurer = new(fakes.FakeConfigurer)
		fakeDnsResolvConfigurer = new(fakes.FakeDnsResolvConfigurer)

		hookActioner = &kawasaki.HookActioner{
			Configurer:          fakeConfigurer,
			DnsResolvConfigurer: fakeDnsResolvConfigurer,
		}

		log = lagertest.NewTestLogger("test")
	})

	Context("when action 'create' is provided", func() {
		It("should apply the configuration", func() {
			cfg := kawasaki.NetworkConfig{
				IPTableInstance: "ba-",
				BridgeName:      "nana",
			}
			Expect(hookActioner.Run(log, "create", cfg, "/path/to/nspath")).To(Succeed())

			Expect(fakeConfigurer.ApplyCallCount()).To(Equal(1))
			_, actualCfg, actualNsPath := fakeConfigurer.ApplyArgsForCall(0)
			Expect(actualCfg).To(Equal(cfg))
			Expect(actualNsPath).To(Equal("/path/to/nspath"))
		})

		Context("when applying the configuration fails", func() {
			It("should return the error", func() {
				fakeConfigurer.ApplyReturns(errors.New("I lost my banana"))

				Expect(hookActioner.Run(log, "create", kawasaki.NetworkConfig{}, "/path/to/nspath")).To(MatchError("I lost my banana"))
			})
		})

		It("should configure DNS resolution", func() {
			cfg := kawasaki.NetworkConfig{
				IPTableInstance: "ba-",
				BridgeName:      "nana",
			}
			Expect(hookActioner.Run(log, "create", cfg, "/path/to/nspath")).To(Succeed())

			Expect(fakeDnsResolvConfigurer.ConfigureCallCount()).To(Equal(1))
		})

		Context("when configuring DNS resolution fails", func() {
			It("should return the error", func() {
				fakeDnsResolvConfigurer.ConfigureReturns(errors.New("I lost my banana"))

				Expect(hookActioner.Run(log, "create", kawasaki.NetworkConfig{}, "/path/to/nspath")).To(MatchError("I lost my banana"))
			})
		})
	})

	Context("when action 'destroy' is provided", func() {
		It("should destroy the configuration", func() {
			cfg := kawasaki.NetworkConfig{
				IPTableInstance: "ba-",
				BridgeName:      "nana",
			}
			Expect(hookActioner.Run(log, "destroy", cfg, "/path/to/nspath")).To(Succeed())

			Expect(fakeConfigurer.DestroyCallCount()).To(Equal(1))
		})

		Context("when applying the configuration fails", func() {
			It("should return the error", func() {
				fakeConfigurer.DestroyReturns(errors.New("I lost my banana"))

				Expect(hookActioner.Run(log, "destroy", kawasaki.NetworkConfig{}, "")).To(MatchError("I lost my banana"))
			})
		})
	})
})

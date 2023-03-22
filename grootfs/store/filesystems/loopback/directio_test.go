package loopback_test

import (
	"errors"

	"code.cloudfoundry.org/grootfs/store/filesystems/loopback"
	"code.cloudfoundry.org/grootfs/store/filesystems/loopback/loopbackfakes"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DirectIOEnabler", func() {
	var (
		loSetup *loopbackfakes.FakeLoSetup
		enabler loopback.DirectIOEnabler
	)

	BeforeEach(func() {
		loSetup = new(loopbackfakes.FakeLoSetup)
		loSetup.FindAssociatedLoopDeviceReturns("/dev/loop123", nil)
		enabler = loopback.DirectIOEnabler{LoSetup: loSetup}
	})

	It("enables direct IO on the loopback device", func() {
		Expect(enabler.Configure("some-path")).To(Succeed())

		Expect(loSetup.FindAssociatedLoopDeviceCallCount()).To(Equal(1))
		actualPath := loSetup.FindAssociatedLoopDeviceArgsForCall(0)
		Expect(actualPath).To(Equal("some-path"))

		Expect(loSetup.EnableDirectIOCallCount()).To(Equal(1))
		actualLoopDev := loSetup.EnableDirectIOArgsForCall(0)
		Expect(actualLoopDev).To(Equal("/dev/loop123"))
	})

	When("finding associated loopback device fails", func() {
		BeforeEach(func() {
			loSetup.FindAssociatedLoopDeviceReturns("", errors.New("lo-error"))
		})

		It("returns the error", func() {
			Expect(enabler.Configure("some-path")).To(MatchError("lo-error"))
		})
	})

	When("enabling direct IO fails", func() {
		BeforeEach(func() {
			loSetup.EnableDirectIOReturns(errors.New("lo-error"))
		})

		It("returns the error", func() {
			Expect(enabler.Configure("some-path")).To(MatchError("lo-error"))
		})
	})
})

var _ = Describe("DirectIODisabler", func() {
	var (
		loSetup  *loopbackfakes.FakeLoSetup
		disabler loopback.DirectIODisabler
	)

	BeforeEach(func() {
		loSetup = new(loopbackfakes.FakeLoSetup)
		loSetup.FindAssociatedLoopDeviceReturns("/dev/loop123", nil)
		disabler = loopback.DirectIODisabler{LoSetup: loSetup}
	})

	It("disables direct IO on the loopback device", func() {
		Expect(disabler.Configure("some-path")).To(Succeed())

		Expect(loSetup.FindAssociatedLoopDeviceCallCount()).To(Equal(1))
		actualPath := loSetup.FindAssociatedLoopDeviceArgsForCall(0)
		Expect(actualPath).To(Equal("some-path"))

		Expect(loSetup.DisableDirectIOCallCount()).To(Equal(1))
		actualLoopDev := loSetup.DisableDirectIOArgsForCall(0)
		Expect(actualLoopDev).To(Equal("/dev/loop123"))
	})

	When("finding associated loopback device fails", func() {
		BeforeEach(func() {
			loSetup.FindAssociatedLoopDeviceReturns("", errors.New("lo-error"))
		})

		It("returns the error", func() {
			Expect(disabler.Configure("some-path")).To(MatchError("lo-error"))
		})
	})

	When("disabling direct IO fails", func() {
		BeforeEach(func() {
			loSetup.DisableDirectIOReturns(errors.New("lo-error"))
		})

		It("returns the error", func() {
			Expect(disabler.Configure("some-path")).To(MatchError("lo-error"))
		})
	})
})

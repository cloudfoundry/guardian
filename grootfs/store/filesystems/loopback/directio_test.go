package loopback_test

import (
	"errors"

	"code.cloudfoundry.org/grootfs/store/filesystems/loopback"
	"code.cloudfoundry.org/grootfs/store/filesystems/loopback/loopbackfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DirectIO", func() {
	var (
		loSetup  *loopbackfakes.FakeLoSetup
		directIO loopback.DirectIO
	)

	BeforeEach(func() {
		loSetup = new(loopbackfakes.FakeLoSetup)
		loSetup.FindAssociatedLoopDeviceReturns("/dev/loop123", nil)
		directIO = loopback.DirectIO{LoSetup: loSetup}
	})

	It("enables direct IO on the loopback device", func() {
		Expect(directIO.EnableDirectIO("some-path")).To(Succeed())

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
			Expect(directIO.EnableDirectIO("some-path")).To(MatchError("lo-error"))
		})
	})

	When("enabling direct IO fails", func() {
		BeforeEach(func() {
			loSetup.EnableDirectIOReturns(errors.New("lo-error"))
		})

		It("returns the error", func() {
			Expect(directIO.EnableDirectIO("some-path")).To(MatchError("lo-error"))
		})
	})
})

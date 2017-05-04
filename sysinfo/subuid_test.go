package sysinfo_test

import (
	"code.cloudfoundry.org/guardian/sysinfo"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UidCanMapRange", func() {
	Context("when the uid can map the exact provided range", func() {
		It("returns true", func() {
			subidFileContents := "1000:0:10\n"
			Expect(sysinfo.UidCanMapExactRange(subidFileContents, "frank", 1000, 0, 10)).To(BeTrue())
		})
	})

	Context("when the username can map the exact provided range", func() {
		It("returns true", func() {
			subidFileContents := "frank:0:10\n"
			Expect(sysinfo.UidCanMapExactRange(subidFileContents, "frank", 1000, 0, 10)).To(BeTrue())
		})
	})

	Context("when the uid can map some of the provided range", func() {
		It("returns false", func() {
			subidFileContents := "1000:0:10\n"
			Expect(sysinfo.UidCanMapExactRange(subidFileContents, "frank", 1000, 0, 9)).To(BeFalse())
		})
	})

	Context("when the uid can't map the desired range", func() {
		It("returns false", func() {
			subidFileContents := "1000:0:9\n"
			Expect(sysinfo.UidCanMapExactRange(subidFileContents, "frank", 1000, 0, 10)).To(BeFalse())
		})
	})

	Context("when the uid is not found in the subid file", func() {
		It("returns false", func() {
			subidFileContents := "1001:0:11\n"
			Expect(sysinfo.UidCanMapExactRange(subidFileContents, "frank", 1000, 0, 10)).To(BeFalse())
		})
	})
})

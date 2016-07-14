package sysinfo_test

import (
	"code.cloudfoundry.org/guardian/sysinfo"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemInfo", func() {
	var provider sysinfo.Provider

	Describe("TotalMemory", func() {
		BeforeEach(func() {
			provider = sysinfo.NewProvider("/")
		})

		It("provides nonzero memory information", func() {
			totalMemory, err := provider.TotalMemory()
			Expect(err).ToNot(HaveOccurred())

			Expect(totalMemory).To(BeNumerically(">", 0))
		})
	})

	Describe("TotalDisk", func() {
		BeforeEach(func() {
			provider = sysinfo.NewProvider("/")
		})

		It("provides nonzero disk information", func() {
			totalDisk, err := provider.TotalDisk()
			Expect(err).ToNot(HaveOccurred())

			Expect(totalDisk).To(BeNumerically(">", 0))
		})
	})
})

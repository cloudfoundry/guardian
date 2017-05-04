package sysinfo_test

import (
	"code.cloudfoundry.org/guardian/sysinfo"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("", func() {
	var provider sysinfo.ResourcesProvider

	Describe("TotalMemory", func() {
		BeforeEach(func() {
			provider = sysinfo.NewResourcesProvider("/")
		})

		It("provides nonzero memory information", func() {
			totalMemory, err := provider.TotalMemory()
			Expect(err).ToNot(HaveOccurred())

			Expect(totalMemory).To(BeNumerically(">", 0))
		})
	})

	Describe("TotalDisk", func() {
		BeforeEach(func() {
			provider = sysinfo.NewResourcesProvider("/")
		})

		It("provides nonzero disk information", func() {
			totalDisk, err := provider.TotalDisk()
			Expect(err).ToNot(HaveOccurred())

			Expect(totalDisk).To(BeNumerically(">", 0))
		})
	})
})

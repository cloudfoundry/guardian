package gqt_test

import (
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garden API", func() {
	var (
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		client = startGarden()
	})

	AfterEach(func() {
		client.DestroyAndStop()
	})

	Describe("Capacity", func() {
		It("returns json representing capacity", func() {
			result, err := client.Capacity()
			Expect(err).NotTo(HaveOccurred())

			Expect(result.MemoryInBytes).To(BeNumerically(">", 0))
			Expect(result.DiskInBytes).To(BeNumerically(">", 0))
			Expect(result.MaxContainers).To(BeNumerically(">", 0))
		})
	})
})

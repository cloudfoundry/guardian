package gqt_test

import (
	"github.com/cloudfoundry-incubator/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garden API", func() {
	var (
		args   []string
		client *runner.RunningGarden
	)

	BeforeEach(func() {
		args = []string{}
	})

	JustBeforeEach(func() {
		client = startGarden(args...)
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

		Context("when the network pool is /24", func() {
			BeforeEach(func() {
				args = append(args, "--networkPool", "10.254.0.0/24")
			})

			It("returns the capacity of the subnet pool", func() {
				capacity, err := client.Capacity()
				Expect(err).ToNot(HaveOccurred())
				Expect(capacity.MaxContainers).To(Equal(uint64(64)))
			})
		})
	})
})

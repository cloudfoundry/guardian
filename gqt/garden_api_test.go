package gqt_test

import (
	"code.cloudfoundry.org/guardian/gqt/runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garden API", func() {
	var (
		client *runner.RunningGarden
	)

	JustBeforeEach(func() {
		client = runner.Start(config)
	})

	AfterEach(func() {
		Expect(client.DestroyAndStop()).To(Succeed())
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
				config.NetworkPool = "10.254.0.0/24"
			})

			It("returns the capacity of the subnet pool", func() {
				capacity, err := client.Capacity()
				Expect(err).ToNot(HaveOccurred())
				Expect(capacity.MaxContainers).To(Equal(uint64(64)))
			})

			Context("when the capacity is limited by maxContainers", func() {
				BeforeEach(func() {
					config.MaxContainers = uint64ptr(32)
				})

				It("returns the limited capacity from maxContainers", func() {
					capacity, err := client.Capacity()
					Expect(err).ToNot(HaveOccurred())
					Expect(capacity.MaxContainers).To(Equal(uint64(32)))
				})
			})
		})
	})
})

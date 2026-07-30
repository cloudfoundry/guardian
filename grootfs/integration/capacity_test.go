package integration_test

import (
	"code.cloudfoundry.org/grootfs/commands/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Capacity", func() {
	BeforeEach(func() {
		Expect(Runner.SetConfig(config.Config{
			Init: config.Init{
				StoreSizeBytes: 190092,
			},
		})).To(Succeed())
	})

	Describe("Capacity", func() {
		It("emits init store size", func() {
			output, err := Runner.RunSubcommand("capacity")

			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(MatchJSON("{\"capacity\": 190092}"))
		})
	})
})

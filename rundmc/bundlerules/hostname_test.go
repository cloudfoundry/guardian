package bundlerules_test

import (
	"strings"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hostname", func() {
	It("sets the correct hostname in the bundle", func() {
		newBndl, err := bundlerules.Hostname{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
			Hostname: "banana",
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(newBndl.Hostname()).To(Equal("banana"))
	})

	Context("when the hostname is longer than 49 characters", func() {
		It("should use the last 49 characters of it", func() {
			newBndl, err := bundlerules.Hostname{}.Apply(goci.Bundle(), spec.DesiredContainerSpec{
				Hostname: strings.Repeat("banana", 9),
			}, "not-needed-path")
			Expect(err).NotTo(HaveOccurred())

			Expect(newBndl.Hostname()).To(Equal("a" + strings.Repeat("banana", 8)))
		})
	})
})

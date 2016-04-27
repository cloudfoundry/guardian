package bundlerules_test

import (
	"strings"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Hostname", func() {
	It("sets the correct hostname in the bundle", func() {
		newBndl := bundlerules.Hostname{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Hostname: "banana",
		})

		Expect(newBndl.Hostname()).To(Equal("banana"))
	})

	Context("when the hostname is longer than 49 characters", func() {
		It("should use the last 49 characters of it", func() {
			newBndl := bundlerules.Hostname{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
				Hostname: strings.Repeat("banana", 9),
			})

			Expect(newBndl.Hostname()).To(Equal("a" + strings.Repeat("banana", 8)))
		})
	})
})

package bundlerules_test

import (
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
})

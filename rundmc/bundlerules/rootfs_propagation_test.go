package bundlerules_test

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RootFSPropagation", func() {
	It("sets the correct rootfs propagation in the bundle", func() {
		newBndl, err := bundlerules.RootFSPropagation{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			RootFSPropagation: "banana",
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(newBndl.RootFSPropagation()).To(Equal("banana"))
	})
})

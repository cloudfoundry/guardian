package bundlerules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("LimitsRule", func() {
	It("sets the correct memory limit in bundle resources", func() {
		newBndl, err := bundlerules.Windows{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Limits: garden.Limits{
				Memory: garden.MemoryLimits{LimitInBytes: 4096},
			},
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(*(newBndl.Spec.Windows.Resources.Memory.Limit)).To(BeNumerically("==", 4096))
	})
})

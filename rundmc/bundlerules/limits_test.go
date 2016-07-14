package bundlerules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("LimitsRule", func() {
	It("sets the correct memory limit in bundle resources", func() {
		newBndl := bundlerules.Limits{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Limits: garden.Limits{
				Memory: garden.MemoryLimits{LimitInBytes: 4096},
			},
		})

		Expect(*(newBndl.Resources().Memory.Limit)).To(BeNumerically("==", 4096))
		Expect(*(newBndl.Resources().Memory.Swap)).To(BeNumerically("==", 4096))
	})

	It("sets the correct CPU limit in bundle resources", func() {
		newBndl := bundlerules.Limits{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Limits: garden.Limits{
				CPU: garden.CPULimits{LimitInShares: 1},
			},
		})

		Expect(*(newBndl.Resources().CPU.Shares)).To(BeNumerically("==", 1))
	})

	It("does not clobber other fields of the resources sections", func() {
		foo := "foo"
		bndl := goci.Bundle().WithResources(
			&specs.Resources{
				Devices: []specs.DeviceCgroup{{Access: &foo}},
			},
		)

		newBndl := bundlerules.Limits{}.Apply(bndl, gardener.DesiredContainerSpec{
			Limits: garden.Limits{
				Memory: garden.MemoryLimits{LimitInBytes: 4096},
			},
		})

		Expect(*(newBndl.Resources().Memory.Limit)).To(BeNumerically("==", 4096))
		Expect(newBndl.Resources().Devices).To(Equal(bndl.Resources().Devices))
	})
})

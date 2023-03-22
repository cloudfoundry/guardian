package bundlerules_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("WindowsRule", func() {
	var (
		desiredSpec spec.DesiredContainerSpec
		newBndl     goci.Bndl
	)

	BeforeEach(func() {
		desiredSpec = spec.DesiredContainerSpec{
			BaseConfig: specs.Spec{
				Windows: &specs.Windows{
					LayerFolders: []string{"layer-1", "layer-0"},
				},
			},
			Limits: garden.Limits{
				Memory: garden.MemoryLimits{LimitInBytes: 4096},
				CPU:    garden.CPULimits{LimitInShares: 321},
			},
		}
	})

	JustBeforeEach(func() {
		var err error
		newBndl, err = bundlerules.Windows{}.Apply(goci.Bundle(), desiredSpec)
		Expect(err).NotTo(HaveOccurred())
	})

	It("copies the Windows config from the BaseConfig and sets the memory limit + cpu shares", func() {
		Expect(*newBndl.Spec.Windows).To(Equal(specs.Windows{
			LayerFolders: []string{"layer-1", "layer-0"},
			Resources: &specs.WindowsResources{
				Memory: &specs.WindowsMemoryResources{
					Limit: uint64ptr(4096),
				},
				CPU: &specs.WindowsCPUResources{
					Shares: uint16ptr(321),
				},
			},
		}))
	})

	When("cpu limit is given as a weight", func() {
		BeforeEach(func() {
			desiredSpec.Limits.CPU.Weight = 432
		})

		It("sets the weight as the cpu shares", func() {
			Expect(*newBndl.Spec.Windows.Resources.CPU.Shares).To(Equal(uint16(432)))
		})
	})

	When("the base bundle does not contain Windows config", func() {
		BeforeEach(func() {
			desiredSpec.BaseConfig.Windows = nil
		})

		It("returns the original bundle", func() {
			Expect(newBndl).To(Equal(goci.Bundle()))
		})
	})
})

func uint64ptr(i uint64) *uint64 {
	return &i
}

func uint16ptr(i uint16) *uint16 {
	return &i
}

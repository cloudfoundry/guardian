package bundlerules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("WindowsRule", func() {
	It("copies the Windows config from the BaseConfig and sets the memory limit", func() {
		layerFolders := []string{"layer-1", "layer-0"}
		newBndl, err := bundlerules.Windows{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			BaseConfig: specs.Spec{
				Windows: &specs.Windows{
					LayerFolders: layerFolders,
				},
			},
			Limits: garden.Limits{
				Memory: garden.MemoryLimits{LimitInBytes: 4096},
			},
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(*newBndl.Spec.Windows).To(Equal(specs.Windows{
			LayerFolders: layerFolders,
			Resources: &specs.WindowsResources{
				Memory: &specs.WindowsMemoryResources{
					Limit: uint64ptr(4096),
				},
			},
		}))
	})

	Context("when the base bundle does not contain Windows config", func() {
		It("returns the original bundle", func() {
			newBndl, err := bundlerules.Windows{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
				BaseConfig: specs.Spec{
					Windows: nil,
				},
			}, "not-needed-path")

			Expect(err).NotTo(HaveOccurred())
			Expect(newBndl).To(Equal(goci.Bundle()))
		})
	})

})

func uint64ptr(i uint64) *uint64 {
	return &i
}

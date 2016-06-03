package bundlerules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
)

var _ = Describe("Base", func() {
	var (
		privilegeBndl, unprivilegeBndl goci.Bndl

		rule bundlerules.Base
	)

	BeforeEach(func() {
		privilegeBndl = goci.Bndl{}.WithNamespace(goci.NetworkNamespace)
		unprivilegeBndl = goci.Bndl{}.WithNamespace(goci.UserNamespace)

		rule = bundlerules.Base{
			PrivilegedBase:   privilegeBndl,
			UnprivilegedBase: unprivilegeBndl,
		}
	})

	Context("when it is privileged", func() {
		It("should use the correct base", func() {
			retBndl := rule.Apply(goci.Bndl{}, gardener.DesiredContainerSpec{
				Privileged: true,
			})

			Expect(retBndl).To(Equal(privilegeBndl))
		})
	})

	Context("when it is not privileged", func() {
		It("should use the correct base", func() {
			retBndl := rule.Apply(goci.Bndl{}, gardener.DesiredContainerSpec{
				Privileged: false,
			})

			Expect(retBndl).To(Equal(unprivilegeBndl))
		})
	})
})

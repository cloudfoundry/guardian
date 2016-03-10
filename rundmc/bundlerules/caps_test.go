package bundlerules_test

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Privileged Caps Rule", func() {
	var (
		bundle *goci.Bndl
		rule   bundlerules.PrivilegedCaps
	)

	BeforeEach(func() {
		bundle = goci.Bundle().WithCapabilities("CAP_POTATO")
		rule = bundlerules.PrivilegedCaps{}
	})

	It("does not add CAP_SYS_ADMIN in unprivileged containers", func() {
		newBndl := rule.Apply(bundle, gardener.DesiredContainerSpec{})
		Expect(newBndl.Process().Capabilities).To(ConsistOf("CAP_POTATO"))
	})

	It("adds CAP_SYS_ADMIN in privileged containers", func() {
		newBndl := rule.Apply(bundle, gardener.DesiredContainerSpec{Privileged: true})
		Expect(newBndl.Process().Capabilities).To(ConsistOf("CAP_POTATO", "CAP_SYS_ADMIN"))
	})
})

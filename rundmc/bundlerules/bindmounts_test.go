package bundlerules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
)

var _ = Describe("BindMountsRule", func() {
	var newBndl *goci.Bndl

	BeforeEach(func() {
		newBndl = bundlerules.BindMounts{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			BindMounts: []garden.BindMount{
				{
					SrcPath: "/path/to/ro/src",
					DstPath: "/path/to/ro/dest",
					Mode:    garden.BindMountModeRO,
				},
				{
					SrcPath: "/path/to/rw/src",
					DstPath: "/path/to/rw/dest",
					Mode:    garden.BindMountModeRW,
				},
			},
		})
	})

	It("adds mounts in the bundle spec", func() {
		Expect(newBndl.Spec.Mounts).To(HaveLen(2))
		Expect(newBndl.Spec.Mounts[0].Path).To(Equal("/path/to/ro/dest"))
		Expect(newBndl.Spec.Mounts[1].Path).To(Equal("/path/to/rw/dest"))
	})

	It("uses the same names for the mounts in the runtime spec", func() {
		mountAName := newBndl.Spec.Mounts[0].Name
		mountBName := newBndl.Spec.Mounts[1].Name

		Expect(newBndl.RuntimeSpec.Mounts).To(HaveKey(mountAName))
		Expect(newBndl.RuntimeSpec.Mounts).To(HaveKey(mountBName))
		Expect(newBndl.RuntimeSpec.Mounts[mountBName]).NotTo(Equal(newBndl.RuntimeSpec.Mounts[mountAName]))
	})

	It("sets the correct runtime spec mount options", func() {
		mountAName := newBndl.Spec.Mounts[0].Name
		mountBName := newBndl.Spec.Mounts[1].Name

		Expect(newBndl.RuntimeSpec.Mounts[mountAName]).To(Equal(specs.Mount{
			Type:    "bind",
			Source:  "/path/to/ro/src",
			Options: []string{"bind", "ro"},
		}))

		Expect(newBndl.RuntimeSpec.Mounts[mountBName]).To(Equal(specs.Mount{
			Type:    "bind",
			Source:  "/path/to/rw/src",
			Options: []string{"bind", "rw"},
		}))
	})
})

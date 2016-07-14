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

var _ = Describe("BindMountsRule", func() {
	var newBndl goci.Bndl

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
		Expect(newBndl.Mounts()).To(HaveLen(2))

		Expect(newBndl.Mounts()).To(ContainElement(specs.Mount{
			Destination: "/path/to/ro/dest",
			Type:        "bind",
			Source:      "/path/to/ro/src",
			Options:     []string{"bind", "ro"},
		}))

		Expect(newBndl.Mounts()).To(ContainElement(specs.Mount{
			Destination: "/path/to/rw/dest",
			Type:        "bind",
			Source:      "/path/to/rw/src",
			Options:     []string{"bind", "rw"},
		}))
	})
})

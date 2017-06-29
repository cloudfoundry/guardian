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

var _ = Describe("MountsRule", func() {
	var newBndl goci.Bndl

	BeforeEach(func() {
		var err error
		newBndl, err = bundlerules.Mounts{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
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
			DesiredImageSpecMounts: []specs.Mount{
				{
					Source:      "src",
					Destination: "dest",
					Options:     []string{"opts"},
					Type:        "mounty",
				},
			},
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())
	})

	It("adds mounts in the bundle spec", func() {
		Expect(newBndl.Mounts()).To(Equal(
			[]specs.Mount{
				{
					Destination: "/path/to/ro/dest",
					Type:        "bind",
					Source:      "/path/to/ro/src",
					Options:     []string{"bind", "ro"},
				},
				{
					Destination: "/path/to/rw/dest",
					Type:        "bind",
					Source:      "/path/to/rw/src",
					Options:     []string{"bind", "rw"},
				},
				{
					Source:      "src",
					Destination: "dest",
					Options:     []string{"opts"},
					Type:        "mounty",
				},
			},
		))
	})
})

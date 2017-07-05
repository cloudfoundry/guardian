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
	var bndl goci.Bndl

	BeforeEach(func() {
		var err error

		preConfiguredMounts := []specs.Mount{
			{
				Destination: "/path/to/dest",
				Source:      "/path/to/src",
				Type:        "preconfigured-mount",
			},
		}
		bindMounts := []garden.BindMount{
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
		}
		desiredImageSpecMounts := []specs.Mount{
			{
				Source:      "src",
				Destination: "dest",
				Options:     []string{"opts"},
				Type:        "mounty",
			},
		}

		originalBndl := goci.Bundle().WithMounts(preConfiguredMounts...)

		bndl, err = bundlerules.Mounts{}.Apply(
			originalBndl,
			gardener.DesiredContainerSpec{
				BindMounts:             bindMounts,
				DesiredImageSpecMounts: desiredImageSpecMounts,
			}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())
	})

	It("adds mounts to the bundle, ensuring desiredImageSpecMounts appear first", func() {
		Expect(bndl.Mounts()[0]).To(Equal(
			specs.Mount{
				Source:      "src",
				Destination: "dest",
				Options:     []string{"opts"},
				Type:        "mounty",
			},
		))
	})

	It("adds mounts to the bundle, ensuring preConfiguredMounts appear second", func() {
		Expect(bndl.Mounts()[1]).To(Equal(
			specs.Mount{
				Source:      "/path/to/src",
				Destination: "/path/to/dest",
				Type:        "preconfigured-mount",
			},
		))
	})

	It("adds mounts to the bundle, ensuring bindMounts appear last", func() {
		Expect(bndl.Mounts()[2]).To(Equal(
			specs.Mount{
				Source:      "/path/to/ro/src",
				Destination: "/path/to/ro/dest",
				Options:     []string{"bind", "ro"},
				Type:        "bind",
			},
		))

		Expect(bndl.Mounts()[3]).To(Equal(
			specs.Mount{
				Source:      "/path/to/rw/src",
				Destination: "/path/to/rw/dest",
				Options:     []string{"bind", "rw"},
				Type:        "bind",
			},
		))
	})
})

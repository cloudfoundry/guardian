package bundlerules_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("MountsRule", func() {
	var (
		logger             *lagertest.TestLogger
		bndl               goci.Bndl
		bundleApplyErr     error
		originalBndl       goci.Bndl
		mountOptionsGetter *rundmcfakes.FakeMountOptionsGetter

		bindMounts             []garden.BindMount
		desiredImageSpecMounts []specs.Mount
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("MountsRule")
		mountOptionsGetter = new(rundmcfakes.FakeMountOptionsGetter)

		preConfiguredMounts := []specs.Mount{
			{
				Destination: "/path/to/dest",
				Source:      "/path/to/src",
				Type:        "preconfigured-mount",
			},
		}
		bindMounts = []garden.BindMount{
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
		desiredImageSpecMounts = []specs.Mount{
			{
				Source:      "src",
				Destination: "dest",
				Options:     []string{"opts"},
				Type:        "mounty",
			},
		}

		originalBndl = goci.Bundle().WithMounts(preConfiguredMounts...)
	})

	JustBeforeEach(func() {
		rule := bundlerules.NewMounts(logger, mountOptionsGetter.Spy)
		bndl, bundleApplyErr = rule.Apply(
			originalBndl,
			spec.DesiredContainerSpec{
				BindMounts: bindMounts,
				BaseConfig: specs.Spec{Mounts: desiredImageSpecMounts},
			})
	})

	Context("when the source is a mountpoint", func() {
		BeforeEach(func() {
			mountOptionsGetter.Returns([]string{"rw", "noexec"}, nil)
		})

		It("checks mount options for the source path", func() {
			Expect(mountOptionsGetter.CallCount()).To(Equal(2))
			actualPath := mountOptionsGetter.ArgsForCall(0)
			Expect(actualPath).To(Equal("/path/to/ro/src"))

			actualPath = mountOptionsGetter.ArgsForCall(1)
			Expect(actualPath).To(Equal("/path/to/rw/src"))
		})

		Context("when checking mount options for the source path fails", func() {
			BeforeEach(func() {
				mountOptionsGetter.Returns(nil, errors.New("options-check-failure"))
			})

			It("logs a warning", func() {
				Expect(logger.LogMessages()).To(ContainElement(ContainSubstring("failed to get mount options")))
			})

			It("ignores the error", func() {
				Expect(bundleApplyErr).NotTo(HaveOccurred())
			})

			It("assumes no additional mount options", func() {
				Expect(bndl.Mounts()).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"Destination": Equal("/path/to/ro/dest"),
						"Options":     ConsistOf("bind", "ro"),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Destination": Equal("/path/to/rw/dest"),
						"Options":     ConsistOf("bind", "rw"),
					}),
				))
			})
		})

		It("preserves source mount options on the bind mount", func() {
			Expect(bndl.Mounts()).To(ContainElements(
				MatchFields(IgnoreExtras, Fields{
					"Source":      Equal("/path/to/ro/src"),
					"Destination": Equal("/path/to/ro/dest"),
					"Options":     ConsistOf("bind", "ro", "noexec"),
					"Type":        Equal("bind"),
				}),
			))
		})

		Context("when multiple mount options are to be filtered out", func() {
			BeforeEach(func() {
				mountOptionsGetter.Returns([]string{"rw", "rw", "noexec", "rw"}, nil)
			})

			It("filters them all", func() {
				Expect(bndl.Mounts()).To(ContainElements(
					MatchFields(IgnoreExtras, Fields{
						"Source":      Equal("/path/to/ro/src"),
						"Destination": Equal("/path/to/ro/dest"),
						"Options":     ConsistOf("bind", "ro", "noexec"),
						"Type":        Equal("bind"),
					}),
				))
			})
		})
	})

	It("succeeds", func() {
		Expect(bundleApplyErr).NotTo(HaveOccurred())
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
		Expect(bndl.Mounts()).To(HaveLen(4))
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

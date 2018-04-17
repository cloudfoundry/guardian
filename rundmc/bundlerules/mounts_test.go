package bundlerules_test

import (
	"errors"

	"github.com/docker/docker/pkg/mount"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/garden"
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
)

var _ = Describe("MountsRule", func() {
	var (
		bndl               goci.Bndl
		bundleApplyErr     error
		originalBndl       goci.Bndl
		mountOptionsGetter *rundmcfakes.FakeMountOptionsGetter

		bindMounts             []garden.BindMount
		desiredImageSpecMounts []specs.Mount
		mountInfosProvider     func() ([]*mount.Info, error)
	)

	BeforeEach(func() {
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

		mountInfosProvider = func() ([]*mount.Info, error) {
			return []*mount.Info{&mount.Info{ID: 1234}}, nil
		}
	})

	JustBeforeEach(func() {
		rule := bundlerules.Mounts{
			MountOptionsGetter: mountOptionsGetter.Spy,
			MountInfosProvider: mountInfosProvider,
		}
		bndl, bundleApplyErr = rule.Apply(
			originalBndl,
			spec.DesiredContainerSpec{
				BindMounts: bindMounts,
				BaseConfig: specs.Spec{Mounts: desiredImageSpecMounts},
			}, "not-needed-path")
	})

	Context("when the source is a mountpoint", func() {
		BeforeEach(func() {
			mountOptionsGetter.Returns([]string{"rw", "noexec"}, nil)
		})

		It("checks mount options for the source path", func() {
			Expect(mountOptionsGetter.CallCount()).To(Equal(2))
			actualPath, mountInfos := mountOptionsGetter.ArgsForCall(0)
			Expect(actualPath).To(Equal("/path/to/ro/src"))
			Expect(mountInfos[0].ID).To(Equal(1234))

			actualPath, mountInfos = mountOptionsGetter.ArgsForCall(1)
			Expect(actualPath).To(Equal("/path/to/rw/src"))
			Expect(mountInfos[0].ID).To(Equal(1234))
		})

		Context("when checking mount options for the source path fails", func() {
			BeforeEach(func() {
				mountOptionsGetter.Returns(nil, errors.New("options-check-failure"))
			})

			It("returns an error", func() {
				Expect(bundleApplyErr).To(MatchError("options-check-failure"))
				Expect(bndl).To(Equal(goci.Bndl{}))
			})
		})

		It("preserves source mount options on the bind mount", func() {
			Expect(bndl.Mounts()[2]).To(Equal(
				specs.Mount{
					Source:      "/path/to/ro/src",
					Destination: "/path/to/ro/dest",
					Options:     []string{"bind", "ro", "noexec"},
					Type:        "bind",
				},
			))
		})
	})

	Context("when getting mountpoints information fails", func() {
		BeforeEach(func() {
			mountInfosProvider = func() ([]*mount.Info, error) {
				return nil, errors.New("minfo")
			}
		})

		It("returns an error", func() {
			Expect(bundleApplyErr).To(MatchError("minfo"))
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

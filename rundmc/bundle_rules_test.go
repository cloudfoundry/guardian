package rundmc_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
)

var _ = Describe("BaseTemplateRule", func() {
	var (
		privilegeBndl, unprivilegeBndl *goci.Bndl

		rule rundmc.BaseTemplateRule
	)

	BeforeEach(func() {
		privilegeBndl = goci.Bndl{}.WithNamespace(goci.NetworkNamespace)
		unprivilegeBndl = goci.Bndl{}.WithNamespace(goci.UserNamespace)

		rule = rundmc.BaseTemplateRule{
			PrivilegedBase:   privilegeBndl,
			UnprivilegedBase: unprivilegeBndl,
		}
	})

	Context("when it is privileged", func() {
		It("should use the correct base", func() {
			retBndl := rule.Apply(nil, gardener.DesiredContainerSpec{
				Privileged: true,
			})

			Expect(retBndl).To(Equal(privilegeBndl))
		})
	})

	Context("when it is not privileged", func() {
		It("should use the correct base", func() {
			retBndl := rule.Apply(nil, gardener.DesiredContainerSpec{
				Privileged: false,
			})

			Expect(retBndl).To(Equal(unprivilegeBndl))
		})
	})
})

var _ = Describe("RootFSRule", func() {
	var (
		fakeMkdirChowner *fakes.FakeMkdirChowner
		rule             rundmc.RootFSRule
	)

	BeforeEach(func() {
		fakeMkdirChowner = new(fakes.FakeMkdirChowner)
		rule = rundmc.RootFSRule{
			ContainerRootUID: 999,
			ContainerRootGID: 888,

			MkdirChowner: fakeMkdirChowner,
		}
	})

	It("applies the rootfs to the passed bundle", func() {
		newBndl := rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			RootFSPath: "/path/to/banana/rootfs",
		})

		Expect(newBndl.Spec.Root.Path).To(Equal("/path/to/banana/rootfs"))
	})

	// this is a workaround for our current aufs code not properly changing the
	// ownership of / to container-root. Without this step runC is unable to
	// pivot root in user-namespaced containers.
	Describe("creating the .pivot_root directory", func() {
		It("pre-creates the /.pivot_root directory with the correct ownership", func() {
			rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
				RootFSPath: "/path/to/banana",
			})

			Expect(fakeMkdirChowner.MkdirChownCallCount()).To(Equal(1))
			path, perms, uid, gid := fakeMkdirChowner.MkdirChownArgsForCall(0)
			Expect(path).To(Equal("/path/to/banana/.pivot_root"))
			Expect(perms).To(Equal(os.FileMode(0700)))
			Expect(uid).To(BeEquivalentTo(999))
			Expect(gid).To(BeEquivalentTo(888))
		})
	})
})

var _ = Describe("NetworkHookRule", func() {
	DescribeTable("the envirionment should contain", func(envVar string) {
		rule := rundmc.NetworkHookRule{LogFilePattern: "/path/to/%s.log"}

		newBndl := rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Handle: "fred",
		})

		Expect(newBndl.RuntimeSpec.Hooks.Prestart[0].Env).To(
			ContainElement(envVar),
		)
	},
		Entry("the GARDEN_LOG_FILE path", "GARDEN_LOG_FILE=/path/to/fred.log"),
		Entry("a sensible PATH", "PATH="+os.Getenv("PATH")),
	)

	It("adds the prestart and poststop hooks of the passed bundle", func() {
		newBndl := rundmc.NetworkHookRule{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			NetworkHooks: gardener.Hooks{
				Prestart: gardener.Hook{
					Path: "/path/to/bananas/network",
					Args: []string{"arg", "barg"},
				},
				Poststop: gardener.Hook{
					Path: "/path/to/bananas/network",
					Args: []string{"arg", "barg"},
				},
			},
		})

		Expect(pathAndArgsOf(newBndl.RuntimeSpec.Hooks.Prestart)).To(ContainElement(PathAndArgs{
			Path: "/path/to/bananas/network",
			Args: []string{"arg", "barg"},
		}))

		Expect(pathAndArgsOf(newBndl.RuntimeSpec.Hooks.Poststop)).To(ContainElement(PathAndArgs{
			Path: "/path/to/bananas/network",
			Args: []string{"arg", "barg"},
		}))
	})
})

func pathAndArgsOf(a []specs.Hook) (b []PathAndArgs) {
	for _, h := range a {
		b = append(b, PathAndArgs{h.Path, h.Args})
	}

	return
}

type PathAndArgs struct {
	Path string
	Args []string
}

var _ = Describe("BindMountsRule", func() {
	var newBndl *goci.Bndl

	BeforeEach(func() {
		newBndl = rundmc.BindMountsRule{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
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

var _ = Describe("LimitsRule", func() {
	It("sets the correct memory limit in bundle resources", func() {
		newBndl := rundmc.LimitsRule{}.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Limits: garden.Limits{
				Memory: garden.MemoryLimits{LimitInBytes: 4096},
			},
		})
		runtimeSpec := newBndl.RuntimeSpec.Linux
		Expect(runtimeSpec.Resources.Memory.Limit).To(BeNumerically("==", 4096))
	})
})

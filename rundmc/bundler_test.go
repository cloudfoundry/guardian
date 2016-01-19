package rundmc_test

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	"github.com/cloudfoundry-incubator/guardian/rundmc/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CompositeBundler", func() {
	var bundler rundmc.BundleTemplate

	Context("when there is only one rule", func() {
		var rule *fakes.FakeBundlerRule

		BeforeEach(func() {
			rule = new(fakes.FakeBundlerRule)
			bundler = rundmc.BundleTemplate{
				Rules: []rundmc.BundlerRule{rule},
			}
		})

		It("returns the bundle from the first rule", func() {
			returnedSpec := goci.Bndl{}.WithRootFS("something")
			rule.ApplyStub = func(bndle *goci.Bndl, spec gardener.DesiredContainerSpec) *goci.Bndl {
				Expect(spec.RootFSPath).To(Equal("the-rootfs"))
				return returnedSpec
			}

			result := bundler.Generate(gardener.DesiredContainerSpec{RootFSPath: "the-rootfs"})
			Expect(result).To(Equal(returnedSpec))
		})

		It("passes nil as the bundle to the first rule", func() {
			bundler.Generate(gardener.DesiredContainerSpec{})

			bndl, _ := rule.ApplyArgsForCall(0)
			Expect(bndl).To(BeNil())
		})
	})

	Context("with multiple rules", func() {
		var (
			ruleA, ruleB *fakes.FakeBundlerRule
		)

		BeforeEach(func() {
			ruleA = new(fakes.FakeBundlerRule)
			ruleB = new(fakes.FakeBundlerRule)

			bundler = rundmc.BundleTemplate{
				Rules: []rundmc.BundlerRule{
					ruleA, ruleB,
				},
			}
		})

		It("calls all the rules", func() {
			bundler.Generate(gardener.DesiredContainerSpec{})

			Expect(ruleA.ApplyCallCount()).To(Equal(1))
			Expect(ruleB.ApplyCallCount()).To(Equal(1))
		})

		It("passes the bundle from the first rule to the subsequent rules", func() {
			bndl := goci.Bndl{}.WithMounts(
				goci.Mount{Name: "test_a"},
				goci.Mount{Name: "test_b"},
			)
			ruleA.ApplyReturns(bndl)

			bundler.Generate(gardener.DesiredContainerSpec{})

			Expect(ruleB.ApplyCallCount()).To(Equal(1))
			recBndl, _ := ruleB.ApplyArgsForCall(0)
			Expect(recBndl).To(Equal(bndl))
		})

		It("returns the results of the last rule", func() {
			bndl := goci.Bndl{}.WithMounts(
				goci.Mount{Name: "test_a"},
				goci.Mount{Name: "test_b"},
			)
			ruleB.ApplyReturns(bndl)

			recBndl := bundler.Generate(gardener.DesiredContainerSpec{})
			Expect(recBndl).To(Equal(bndl))
		})
	})
})

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
	It("applies the rootfs to the passed bundle", func() {
		bndl := goci.Bndl{}.WithNamespace(goci.UserNamespace)

		newBndl := rundmc.RootFSRule{}.Apply(bndl, gardener.DesiredContainerSpec{RootFSPath: "/path/to/banana/rootfs"})
		Expect(newBndl.Spec.Root.Path).To(Equal("/path/to/banana/rootfs"))
	})
})

var _ = Describe("NetworkHookRule", func() {
	It("add the hook to the pre-start hooks of the passed bundle", func() {
		bndl := goci.Bndl{}.WithNamespace(goci.UserNamespace)

		newBndl := rundmc.NetworkHookRule{}.Apply(bndl, gardener.DesiredContainerSpec{
			NetworkHook: gardener.Hook{
				Path: "/path/to/bananas/network",
				Args: []string{"arg", "barg"},
			},
		})
		Expect(newBndl.RuntimeSpec.Hooks.Prestart).To(ContainElement(specs.Hook{
			Path: "/path/to/bananas/network",
			Args: []string{"arg", "barg"},
		}))
	})
})

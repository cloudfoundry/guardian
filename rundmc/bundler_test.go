package rundmc_test

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	fakes "code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("BundleTemplate", func() {
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
			rule.ApplyStub = func(bndle goci.Bndl, spec gardener.DesiredContainerSpec) goci.Bndl {
				Expect(spec.RootFSPath).To(Equal("the-rootfs"))
				return returnedSpec
			}

			result := bundler.Generate(gardener.DesiredContainerSpec{RootFSPath: "the-rootfs"})
			Expect(result).To(Equal(returnedSpec))
		})

		It("passes an empty bundle to the first rule", func() {
			bundler.Generate(gardener.DesiredContainerSpec{})

			bndl, _ := rule.ApplyArgsForCall(0)
			Expect(bndl).To(Equal(goci.Bndl{}))
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
				specs.Mount{Destination: "test_a"},
				specs.Mount{Destination: "test_b"},
			)
			ruleA.ApplyReturns(bndl)

			bundler.Generate(gardener.DesiredContainerSpec{})

			Expect(ruleB.ApplyCallCount()).To(Equal(1))
			recBndl, _ := ruleB.ApplyArgsForCall(0)
			Expect(recBndl).To(Equal(bndl))
		})

		It("returns the results of the last rule", func() {
			bndl := goci.Bndl{}.WithMounts(
				specs.Mount{Destination: "test_a"},
				specs.Mount{Destination: "test_b"},
			)
			ruleB.ApplyReturns(bndl)

			recBndl := bundler.Generate(gardener.DesiredContainerSpec{})
			Expect(recBndl).To(Equal(bndl))
		})
	})
})

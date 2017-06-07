package rundmc_test

import (
	"errors"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	fakes "code.cloudfoundry.org/guardian/rundmc/rundmcfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("BundleTemplate", func() {
	var (
		bundler      rundmc.BundleTemplate
		containerDir = "some-container-dir"
	)

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
			rule.ApplyStub = func(bndle goci.Bndl, spec gardener.DesiredContainerSpec, containerDir string) (goci.Bndl, error) {
				Expect(spec.RootFSPath).To(Equal("the-rootfs"))
				return returnedSpec, nil
			}

			result, err := bundler.Generate(gardener.DesiredContainerSpec{RootFSPath: "the-rootfs"}, containerDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(returnedSpec))
		})

		It("returns the error from the first failing rule", func() {
			rule.ApplyReturns(goci.Bndl{}, errors.New("didn't work"))
			_, err := bundler.Generate(gardener.DesiredContainerSpec{RootFSPath: "the-rootfs"}, containerDir)
			Expect(err).To(MatchError(ContainSubstring("didn't work")))
		})

		It("passes an empty bundle, the desired spec, and container dir to the first rule", func() {
			spec := gardener.DesiredContainerSpec{Handle: "some-handle"}
			bundler.Generate(spec, containerDir)

			Expect(rule.ApplyCallCount()).To(Equal(1))
			bndl, actualSpec, actualContainerDir := rule.ApplyArgsForCall(0)
			Expect(bndl).To(Equal(goci.Bndl{}))
			Expect(actualSpec).To(Equal(spec))
			Expect(actualContainerDir).To(Equal(containerDir))
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
			bundler.Generate(gardener.DesiredContainerSpec{}, containerDir)

			Expect(ruleA.ApplyCallCount()).To(Equal(1))
			Expect(ruleB.ApplyCallCount()).To(Equal(1))
		})

		It("passes the bundle from the first rule to the subsequent rules", func() {
			bndl := goci.Bndl{}.WithMounts(
				specs.Mount{Destination: "test_a"},
				specs.Mount{Destination: "test_b"},
			)
			ruleA.ApplyReturns(bndl, nil)

			bundler.Generate(gardener.DesiredContainerSpec{}, containerDir)

			Expect(ruleB.ApplyCallCount()).To(Equal(1))
			recBndl, _, _ := ruleB.ApplyArgsForCall(0)
			Expect(recBndl).To(Equal(bndl))
		})

		It("returns the results of the last rule", func() {
			bndl := goci.Bndl{}.WithMounts(
				specs.Mount{Destination: "test_a"},
				specs.Mount{Destination: "test_b"},
			)
			ruleB.ApplyReturns(bndl, nil)

			recBndl, err := bundler.Generate(gardener.DesiredContainerSpec{}, containerDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(recBndl).To(Equal(bndl))
		})
	})
})

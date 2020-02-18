package bundlerules_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules/bundlerulesfakes"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = Describe("KernelParameter", func() {
	var (
		sysctl              *bundlerulesfakes.FakeSysctl
		kernelParameterRule *bundlerules.KernelParameter
		bundle              goci.Bndl
		err                 error
	)

	BeforeEach(func() {
		sysctl = new(bundlerulesfakes.FakeSysctl)
		kernelParameterRule = bundlerules.NewKernelParameter(sysctl, "foo", 42)
	})

	JustBeforeEach(func() {
		bundle, err = kernelParameterRule.Apply(goci.Bundle(), spec.DesiredContainerSpec{})
	})

	It("succeeds", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("sets the specified kernel parameter to the specified value", func() {
		Expect(bundle.Spec.Linux.Sysctl["foo"]).To(Equal("42"))
	})

	When("the specified value is 0", func() {
		BeforeEach(func() {
			sysctl.GetReturns(42, nil)
			kernelParameterRule = bundlerules.NewKernelParameter(sysctl, "foo", 0)
		})

		It("sets the value to the host one", func() {
			Expect(bundle.Spec.Linux.Sysctl["foo"]).To(Equal("42"))

			Expect(sysctl.GetCallCount()).To(Equal(1))
			Expect(sysctl.GetArgsForCall(0)).To(Equal("foo"))
		})

		When("reading the host value fails", func() {
			BeforeEach(func() {
				sysctl.GetReturns(0, errors.New("sysctl-get-error"))
			})

			It("fails", func() {
				Expect(err).To(MatchError(ContainSubstring("sysctl-get-error")))
			})
		})
	})
})

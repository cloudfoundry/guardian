package privchecker_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"

	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/privchecker"
	"code.cloudfoundry.org/guardian/rundmc/runcontainerd/privchecker/privcheckerfakes"
)

var _ = Describe("PrivilegeChecker", func() {
	var (
		containerManager *privcheckerfakes.FakeContainerManager
		privilegeChecker privchecker.PrivilegeChecker
	)

	BeforeEach(func() {
		containerManager = new(privcheckerfakes.FakeContainerManager)
		privilegeChecker = privchecker.PrivilegeChecker{Log: nil, ContainerManager: containerManager}
	})

	When("the spec includes the user namespace", func() {
		BeforeEach(func() {
			containerManager.SpecReturns(&specs.Spec{
				Linux: &specs.Linux{
					Namespaces: []specs.LinuxNamespace{
						{Type: "something"},
						{Type: "user"},
						{Type: "other"},
					},
				},
			}, nil)
		})

		It("returns false", func() {
			privileged, err := privilegeChecker.Privileged("container-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(privileged).To(BeFalse())

			Expect(containerManager.SpecCallCount()).To(Equal(1))
			_, actualId := containerManager.SpecArgsForCall(0)
			Expect(actualId).To(Equal("container-id"))
		})
	})

	When("the spec does not include the user namespace", func() {
		BeforeEach(func() {
			containerManager.SpecReturns(&specs.Spec{
				Linux: &specs.Linux{
					Namespaces: []specs.LinuxNamespace{
						{Type: "something"},
						{Type: "other"},
					},
				},
			}, nil)
		})

		It("returns true", func() {
			Expect(privilegeChecker.Privileged("container-id")).To(BeTrue())
		})
	})

	When("retrieving the spec fails", func() {
		BeforeEach(func() {
			containerManager.SpecReturns(nil, errors.New("spec-error"))
		})

		It("fails", func() {
			privileged, err := privilegeChecker.Privileged("container-id")

			Expect(privileged).To(BeFalse())
			Expect(err).To(MatchError("spec-error"))
		})
	})
})

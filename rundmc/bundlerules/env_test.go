package bundlerules_test

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Env Rule", func() {
	var (
		userEnv []string
		newBndl goci.Bndl
		rule    bundlerules.Env
	)

	BeforeEach(func() {
		userEnv = []string{"TEST=pineapple", "FOO=bar"}
	})

	JustBeforeEach(func() {
		var err error
		rule = bundlerules.Env{}
		newBndl, err = rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Env: userEnv,
			BaseConfig: specs.Spec{
				Process: &specs.Process{
					Env: []string{
						"TEST=banana",
						"CONTAINER_NAME=hello",
					},
				},
			},
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())
	})

	It("merges the user env with the base config env, with the user env placed last (takes precedence)", func() {
		Expect(newBndl.Spec.Process.Env).To(Equal([]string{
			"TEST=banana", "CONTAINER_NAME=hello", "TEST=pineapple", "FOO=bar",
		}))
	})

	Context("when the user env is nil", func() {
		BeforeEach(func() {
			userEnv = nil
		})

		It("returns the base config env", func() {
			Expect(newBndl.Spec.Process.Env).To(Equal([]string{
				"TEST=banana", "CONTAINER_NAME=hello",
			}))
		})
	})
})

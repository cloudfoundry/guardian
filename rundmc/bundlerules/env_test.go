package bundlerules_test

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Env Rule", func() {
	var (
		newBndl goci.Bndl
		rule    bundlerules.Env
	)

	JustBeforeEach(func() {
		rule = bundlerules.Env{}
		newBndl = rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Env: []string{
				"TEST=banana",
				"CONTAINER_NAME=hello",
			},
		})
	})

	It("sets the environment onto the bundle process", func() {
		Expect(newBndl.Spec.Process.Env).To(Equal([]string{
			"TEST=banana", "CONTAINER_NAME=hello",
		}))
	})
})

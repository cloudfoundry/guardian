package bundlerules_test

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InitProcessRule", func() {
	var (
		newBndl *goci.Bndl
		process specs.Process
		env     []string
		rule    bundlerules.InitProcess
	)

	BeforeEach(func() {
		process = specs.Process{
			Env: []string{"ENV_CONTAINER=1"},
		}

		env = []string{}
	})

	JustBeforeEach(func() {
		rule = bundlerules.InitProcess{
			Process: process,
		}

		newBndl = rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Env: env,
		})
	})

	It("adds the injected init process in the bundle spec", func() {
		Expect(newBndl.Spec.Process).To(Equal(process))
	})

	Context("when environment variables are set in the desired container spec", func() {
		BeforeEach(func() {
			env = []string{
				"TEST=banana",
				"CONTAINER_NAME=hello",
			}
		})

		It("appends the provided environment variables in the bundle spec", func() {
			Expect(newBndl.Spec.Process.Env).To(Equal([]string{
				"ENV_CONTAINER=1", "TEST=banana", "CONTAINER_NAME=hello",
			}))
		})

		Context("when the rule is reused", func() {
			It("should not keep the old environment variables", func() {
				newEnv := []string{
					"FRUIT=banana",
					"TERM=xterm",
				}
				newNewBndl := rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
					Env: newEnv,
				})

				Expect(newNewBndl.Spec.Process.Env).To(Equal([]string{
					"ENV_CONTAINER=1", "FRUIT=banana", "TERM=xterm",
				}))
			})
		})
	})
})

package runrunc_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	"code.cloudfoundry.org/guardian/rundmc/runrunc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("WindowsEnv", func() {
	It("appends the spec env to the bundle env", func() {
		process := specs.Process{Env: []string{"FOO=bar"}}
		actualEnv := runrunc.WindowsEnvFor(5, goci.Bundle().WithProcess(process), garden.ProcessSpec{Env: []string{"BAZ=barry"}})

		Expect(actualEnv).To(Equal([]string{"FOO=bar", "BAZ=barry"}))
	})
})

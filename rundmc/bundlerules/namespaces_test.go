package bundlerules_test

import (
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Namespaces", func() {
	It("sets all namespaces in the bundle to those provided by the spec", func() {
		initialBndl := goci.Bundle()

		desiredContainerSpec := gardener.DesiredContainerSpec{Namespaces: map[string]string{"mount": "", "network": "test-net-ns", "user": "test-user-ns"}}
		transformedBndl, err := bundlerules.Namespaces{}.Apply(initialBndl, desiredContainerSpec, "")
		Expect(err).NotTo(HaveOccurred())

		Expect(transformedBndl.Namespaces()).To(ConsistOf(
			specs.LinuxNamespace{Type: "mount"},
			specs.LinuxNamespace{Type: "network", Path: "test-net-ns"},
			specs.LinuxNamespace{Type: "user", Path: "test-user-ns"},
		))
	})
})

package bundlerules_test

import (
	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

var _ = Describe("Namespaces", func() {
	It("sets all namespaces in the bundle to those provided by the spec", func() {
		initialBndl := goci.Bundle()

		desiredContainerSpec := spec.DesiredContainerSpec{Namespaces: map[string]string{"mount": "", "network": "test-net-ns", "user": "test-user-ns"}}
		transformedBndl, err := bundlerules.Namespaces{}.Apply(initialBndl, desiredContainerSpec)
		Expect(err).NotTo(HaveOccurred())

		Expect(transformedBndl.Namespaces()).To(ConsistOf(
			specs.LinuxNamespace{Type: "mount"},
			specs.LinuxNamespace{Type: "network", Path: "test-net-ns"},
			specs.LinuxNamespace{Type: "user", Path: "test-user-ns"},
		))
	})
})

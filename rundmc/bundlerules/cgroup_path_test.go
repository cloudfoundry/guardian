package bundlerules_test

import (
	"path/filepath"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CGroup Path", func() {
	It("sets the correct cgroup path in the bundle for unprivileged containers", func() {
		cgroupPathRule := bundlerules.CGroupPath{
			Path: "unpriv",
		}

		newBndl, err := cgroupPathRule.Apply(goci.Bundle(), spec.DesiredContainerSpec{
			Handle: "banana",
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(newBndl.CGroupPath()).To(Equal(filepath.Join("unpriv", "banana")))
	})

	It("sets the correct cgroup path in the bundle for privileged containers", func() {
		cgroupPathRule := bundlerules.CGroupPath{
			Path: "unpriv",
		}

		newBndl, err := cgroupPathRule.Apply(goci.Bundle(), spec.DesiredContainerSpec{
			Privileged: true,
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())
		Expect(newBndl.CGroupPath()).To(BeEmpty())
	})
})

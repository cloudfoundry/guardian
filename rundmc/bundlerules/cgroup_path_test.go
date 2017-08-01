package bundlerules_test

import (
	"path/filepath"

	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CGroup Path", func() {
	It("sets the correct garden cgroup path in the bundle", func() {
		cgroupPathRule := bundlerules.CGroupPath{
			GardenCgroup: "test-garden-cgroup",
		}

		newBndl, err := cgroupPathRule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			Hostname: "banana",
		}, "not-needed-path")
		Expect(err).NotTo(HaveOccurred())

		Expect(newBndl.CGroupPath()).To(Equal(filepath.Join("test-garden-cgroup", "banana")))
	})
})

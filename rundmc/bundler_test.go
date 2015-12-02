package rundmc_test

import (
	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/goci/specs"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bundle", func() {
	Context("when there is a network path", func() {
		It("adds the network path to the network namespace of the bundle", func() {
			base := goci.Bundle().WithNamespaces(specs.Namespace{Type: "network"})
			modifiedBundle := rundmc.BundleTemplate{Bndl: base}.Bundle(gardener.DesiredContainerSpec{
				NetworkPath: "/path/to/network",
			})

			Expect(modifiedBundle.RuntimeSpec.Linux.Namespaces).Should(ConsistOf(
				specs.Namespace{Type: specs.NetworkNamespace, Path: "/path/to/network"},
			))
		})
	})

	Context("when there is a root filesystem path", func() {
		It("adds the rootfs path to the bundle", func() {
			base := goci.Bundle()
			modifiedBundle := rundmc.BundleTemplate{Bndl: base}.Bundle(gardener.DesiredContainerSpec{
				RootFSPath: "/some/path",
			})

			Expect(modifiedBundle.Spec.Root.Path).To(Equal("/some/path"))
		})
	})

	It("does not modify the other fields", func() {
		base := goci.Bundle().WithProcess(goci.Process("potato"))
		modifiedBundle := rundmc.BundleTemplate{Bndl: base}.Bundle(gardener.DesiredContainerSpec{})
		Expect(modifiedBundle.Spec.Process.Args).Should(ConsistOf("potato"))
	})
})

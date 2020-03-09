package bundlerules_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/runtime-spec/specs-go"

	spec "code.cloudfoundry.org/guardian/gardener/container-spec"
	"code.cloudfoundry.org/guardian/rundmc/bundlerules"
	"code.cloudfoundry.org/guardian/rundmc/goci"
)

var _ = FDescribe("Base", func() {
	var (
		bundle   goci.Bndl
		err      error
		initPath = "/foo/init"

		rule        bundlerules.Init
		desiredSpec spec.DesiredContainerSpec
	)

	BeforeEach(func() {
		rule = bundlerules.Init{InitPath: initPath}
	})

	JustBeforeEach(func() {
		bundle, err = rule.Apply(goci.Bundle(), desiredSpec)
	})

	Context("when it is a normal container", func() {
		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets garden-init as the bundle init process", func() {
			Expect(bundle.Process().Args).To(Equal([]string{"/tmp/garden-init"}))
		})

		It("appends the garden-init bind mount to the bundle mounts", func() {
			Expect(bundle.Mounts()).To(ContainElement(specs.Mount{
				Destination: "/tmp/garden-init",
				Source:      initPath,
				Type:        "bind",
			}))
		})
	})

	Context("appends the -init bind mount to the bundle mounts", func() {
		BeforeEach(func() {
			desiredSpec = spec.DesiredContainerSpec{
				SandboxHandle: "some-handle",
			}
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets garden-init as the bundle init process", func() {
			Expect(bundle.Process().Args).To(Equal([]string{"/tmp/garden-pea-init"}))
		})

		It("appends the garden-pea-init bind mount to the bundle mounts", func() {
			Expect(bundle.Mounts()).To(ContainElement(specs.Mount{
				Destination: "/tmp/garden-pea-init",
				Source:      initPath,
				Type:        "bind",
			}))
		})
	})
})

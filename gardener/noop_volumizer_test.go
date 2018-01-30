package gardener_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NoopVolumizer", func() {
	var volumizer gardener.NoopVolumizer

	var logger *lagertest.TestLogger

	BeforeEach(func() {
		volumizer = gardener.NoopVolumizer{}

		logger = lagertest.NewTestLogger("test")
	})

	Describe("Create", func() {
		It("returns ErrGraphDisabled", func() {
			_, err := volumizer.Create(logger, "some-handle", rootfs_spec.Spec{})
			Expect(err).To(Equal(gardener.ErrGraphDisabled))
		})
	})

	Describe("Destroy", func() {
		It("succeeds, as destroying is idempotent and may be actually called redundantly", func() {
			Expect(volumizer.Destroy(logger, "some-handle")).To(BeNil())
		})
	})

	Describe("Metrics", func() {
		It("successfully returns an empty set of metrics", func() {
			Expect(volumizer.Metrics(logger, "some-handle", false)).To(Equal(garden.ContainerDiskStat{}))
		})
	})

	Describe("GC", func() {
		It("succeeds", func() {
			Expect(volumizer.GC(logger)).To(BeNil())
		})
	})
})

package gardener_test

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden-shed/rootfs_provider"
	"code.cloudfoundry.org/guardian/gardener"
	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NoopVolumeCreator", func() {
	var volumeCreator gardener.NoopVolumeCreator

	var logger *lagertest.TestLogger

	BeforeEach(func() {
		volumeCreator = gardener.NoopVolumeCreator{}

		logger = lagertest.NewTestLogger("test")
	})

	Describe("Create", func() {
		It("returns ErrGraphDisabled", func() {
			_, _, err := volumeCreator.Create(logger, "some-handle", rootfs_provider.Spec{})
			Expect(err).To(Equal(gardener.ErrGraphDisabled))
		})
	})

	Describe("Destroy", func() {
		It("succeeds, as destroying is idempotent and may be actually called redundantly", func() {
			Expect(volumeCreator.Destroy(logger, "some-handle")).To(BeNil())
		})
	})

	Describe("Metrics", func() {
		It("successfully returns an empty set of metrics", func() {
			Expect(volumeCreator.Metrics(logger, "some-handle")).To(Equal(garden.ContainerDiskStat{}))
		})
	})

	Describe("GC", func() {
		It("succeeds", func() {
			Expect(volumeCreator.GC(logger)).To(BeNil())
		})
	})
})

package imageplugin_test

import (
	"errors"

	"code.cloudfoundry.org/garden-shed/rootfs_spec"
	"code.cloudfoundry.org/guardian/imageplugin"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NotImplementedCommandCreator", func() {
	var (
		notImplementedCommandCreator *imageplugin.NotImplementedCommandCreator
	)

	BeforeEach(func() {
		notImplementedCommandCreator = &imageplugin.NotImplementedCommandCreator{
			Err: errors.New("NOT IMPLEMENTED"),
		}
	})

	Describe("CreateCommand", func() {
		It("returns nil and provided error", func() {
			cmd, err := notImplementedCommandCreator.CreateCommand(nil, "", rootfs_spec.Spec{})
			Expect(cmd).To(BeNil())
			Expect(err).To(MatchError(errors.New("NOT IMPLEMENTED")))
		})
	})

	Describe("DestroyCommand", func() {
		It("returns nil", func() {
			Expect(notImplementedCommandCreator.DestroyCommand(nil, "")).To(BeNil())
		})
	})

	Describe("MetricsCommand", func() {
		It("returns nil", func() {
			Expect(notImplementedCommandCreator.MetricsCommand(nil, "")).To(BeNil())
		})
	})
})

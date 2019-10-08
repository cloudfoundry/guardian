package mount_test

import (
	"code.cloudfoundry.org/grootfs/store/filesystems/mount"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NilUnmounter", func() {
	var unmounter mount.NilUnmounter

	BeforeEach(func() {
		unmounter = mount.NilUnmounter{}
	})

	It("panics when asked to unmount", func() {
		Expect(func() { unmounter.Unmount("/foo/bar") }).To(Panic())
	})
})

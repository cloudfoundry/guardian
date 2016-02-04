package bundlerules_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/goci"
	"github.com/cloudfoundry-incubator/guardian/gardener"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules"
	"github.com/cloudfoundry-incubator/guardian/rundmc/bundlerules/fakes"
)

var _ = Describe("RootFS", func() {
	var (
		fakeMkdirChowner *fakes.FakeMkdirChowner
		fakeDirRemover   *fakes.FakeDirRemover
		rule             bundlerules.RootFS
	)

	BeforeEach(func() {
		fakeMkdirChowner = new(fakes.FakeMkdirChowner)
		fakeDirRemover = new(fakes.FakeDirRemover)
		rule = bundlerules.RootFS{
			ContainerRootUID: 999,
			ContainerRootGID: 888,

			MkdirChowner: fakeMkdirChowner,
			DirRemover:   fakeDirRemover,
		}
	})

	It("applies the rootfs to the passed bundle", func() {
		newBndl := rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			RootFSPath: "/path/to/banana/rootfs",
		})

		Expect(newBndl.Spec.Root.Path).To(Equal("/path/to/banana/rootfs"))
	})

	// this is a workaround for our current aufs code not properly changing the
	// ownership of / to container-root. Without this step runC is unable to
	// pivot root in user-namespaced containers.
	Describe("creating the .pivot_root directory", func() {
		It("pre-creates the /.pivot_root directory with the correct ownership", func() {
			rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
				RootFSPath: "/path/to/banana",
			})

			Expect(fakeMkdirChowner.MkdirChownCallCount()).To(Equal(1))
			path, perms, uid, gid := fakeMkdirChowner.MkdirChownArgsForCall(0)
			Expect(path).To(Equal("/path/to/banana/.pivot_root"))
			Expect(perms).To(Equal(os.FileMode(0700)))
			Expect(uid).To(BeEquivalentTo(999))
			Expect(gid).To(BeEquivalentTo(888))
		})
	})

	// this is workaround for the /dev/shm existing before the container creation
	It("deletes the /dev/shm so that it can be created by runc with correct permissions", func() {
		rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			RootFSPath: "/path/to/banana",
		})

		Expect(fakeDirRemover.RemoveCallCount()).To(Equal(1))

		name := fakeDirRemover.RemoveArgsForCall(0)
		Expect(name).To(Equal("/path/to/banana/dev/shm"))
	})
})

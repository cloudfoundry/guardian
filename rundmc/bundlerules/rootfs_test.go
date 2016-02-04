package bundlerules_test

import (
	"io/ioutil"
	"os"
	"path"

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
		rule             bundlerules.RootFS

		rootfsPath     string
		returnedBundle *goci.Bndl
	)

	BeforeEach(func() {
		fakeMkdirChowner = new(fakes.FakeMkdirChowner)
		rootfsPath = tmp()

		rule = bundlerules.RootFS{
			ContainerRootUID: 999,
			ContainerRootGID: 888,

			MkdirChowner: fakeMkdirChowner,
		}

		Expect(os.MkdirAll(path.Join(rootfsPath, "dev", "shm"), 0700)).To(Succeed())
		Expect(ioutil.WriteFile(path.Join(rootfsPath, "dev", "foo"), []byte("blah"), 0700)).To(Succeed())
		Expect(os.MkdirAll(path.Join(rootfsPath, "notdev", "shm"), 0700)).To(Succeed())

		returnedBundle = rule.Apply(goci.Bundle(), gardener.DesiredContainerSpec{
			RootFSPath: rootfsPath,
		})
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootfsPath)).To(Succeed())
	})

	It("applies the rootfs to the passed bundle", func() {
		Expect(returnedBundle.Spec.Root.Path).To(Equal(rootfsPath))
	})

	// this is a workaround for our current aufs code not properly changing the
	// ownership of / to container-root. Without this step runC is unable to
	// pivot root in user-namespaced containers.
	Describe("creating the .pivot_root directory", func() {
		It("pre-creates the /.pivot_root directory with the correct ownership", func() {
			p, perms, uid, gid := fakeMkdirChowner.MkdirChownArgsForCall(0)
			Expect(p).To(Equal(path.Join(rootfsPath, ".pivot_root")))
			Expect(perms).To(Equal(os.FileMode(0700)))
			Expect(uid).To(BeEquivalentTo(999))
			Expect(gid).To(BeEquivalentTo(888))
		})
	})

	// stuff in this directory frequently confuses runc, and poses a potential
	// security vulnerability.
	It("deletes the /dev/ directory", func() {
		Expect(path.Join(rootfsPath, "dev")).NotTo(BeAnExistingFile())
		Expect(path.Join(rootfsPath, "notdev", "shm")).To(BeAnExistingFile())
	})

	It("recreates /dev as container root", func() {
		p, perms, uid, gid := fakeMkdirChowner.MkdirChownArgsForCall(1)
		Expect(p).To(Equal(path.Join(rootfsPath, "dev")))
		Expect(perms).To(Equal(os.FileMode(0755)))
		Expect(uid).To(BeEquivalentTo(999))
		Expect(gid).To(BeEquivalentTo(888))
	})
})

func tmp() string {
	tmp, err := ioutil.TempDir("", "rootfstest")
	Expect(err).NotTo(HaveOccurred())
	return tmp
}

package base_image_puller_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"code.cloudfoundry.org/commandrunner/linux_command_runner"
	"code.cloudfoundry.org/grootfs/base_image_puller"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/groot/grootfakes"
	"code.cloudfoundry.org/grootfs/sandbox"
	"code.cloudfoundry.org/grootfs/sandbox/sandboxfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BasedirHandler", func() {
	var (
		logger              lager.Logger
		volumeDir           string
		parentLayerPath     string
		childLayerPath      string
		cloneUsernsOnHandle bool
		reexecer            groot.SandboxReexecer

		handler base_image_puller.BaseDirHandler
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		runner := linux_command_runner.New()
		idMapper := new(sandboxfakes.FakeIDMapper)
		cloneUsernsOnHandle = false
		reexecer = sandbox.NewReexecer(logger, runner, idMapper, groot.IDMappings{})

		var err error
		volumeDir, err = ioutil.TempDir("", "volume-")
		Expect(err).NotTo(HaveOccurred())

		parentLayerPath = filepath.Join(volumeDir, "layer-1")
		Expect(os.MkdirAll(parentLayerPath, 0755)).To(Succeed())
		childLayerPath = filepath.Join(volumeDir, "layer-2")
		Expect(os.MkdirAll(childLayerPath, 0755)).To(Succeed())

	})

	JustBeforeEach(func() {
		handler = base_image_puller.NewBasedirHandler(reexecer, cloneUsernsOnHandle)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(volumeDir)).To(Succeed())
	})

	It("creates the base directory in the child path", func() {
		Expect(os.MkdirAll(filepath.Join(parentLayerPath, "foo", "bar"), 0755)).To(Succeed())
		Expect(handler.Handle(logger, base_image_puller.UnpackSpec{TargetPath: childLayerPath, BaseDirectory: "/foo/bar"}, parentLayerPath)).To(Succeed())
		Expect(filepath.Join(childLayerPath, "foo")).To(BeADirectory())
		Expect(filepath.Join(childLayerPath, "foo", "bar")).To(BeADirectory())
	})

	It("preserves parent layer ownership", func() {
		Expect(os.MkdirAll(filepath.Join(parentLayerPath, "foo", "bar"), 0755)).To(Succeed())
		Expect(os.Chown(filepath.Join(parentLayerPath, "foo"), 1000, 1000)).To(Succeed())
		Expect(os.Chown(filepath.Join(parentLayerPath, "foo", "bar"), 2000, 2000)).To(Succeed())

		Expect(handler.Handle(logger, base_image_puller.UnpackSpec{TargetPath: childLayerPath, BaseDirectory: "/foo/bar"}, parentLayerPath)).To(Succeed())

		ownerUid, ownerGid := getOwner(filepath.Join(childLayerPath, "foo"))
		Expect(ownerUid).To(Equal(1000))
		Expect(ownerGid).To(Equal(1000))

		ownerUid, ownerGid = getOwner(filepath.Join(childLayerPath, "foo", "bar"))
		Expect(ownerUid).To(Equal(2000))
		Expect(ownerGid).To(Equal(2000))
	})

	It("preserves parent layer permissions", func() {
		Expect(os.MkdirAll(filepath.Join(parentLayerPath, "foo", "bar"), 0755)).To(Succeed())
		Expect(os.Chmod(filepath.Join(parentLayerPath, "foo"), 0244)).To(Succeed())
		Expect(os.Chmod(filepath.Join(parentLayerPath, "foo", "bar"), 0422)).To(Succeed())

		Expect(handler.Handle(logger, base_image_puller.UnpackSpec{TargetPath: childLayerPath, BaseDirectory: "/foo/bar"}, parentLayerPath)).To(Succeed())

		Expect(getPermissions(filepath.Join(childLayerPath, "foo"))).To(Equal(os.FileMode(0244)))
		Expect(getPermissions(filepath.Join(childLayerPath, "foo", "bar"))).To(Equal(os.FileMode(0422)))
	})

	It("does not clone user namespace", func() {
		reexecer := new(grootfakes.FakeSandboxReexecer)
		handler = base_image_puller.NewBasedirHandler(reexecer, false)
		handler.Handle(logger, base_image_puller.UnpackSpec{TargetPath: childLayerPath, BaseDirectory: "/foo/bar"}, parentLayerPath)

		Expect(reexecer.ReexecCallCount()).To(Equal(1))
		_, reexecSpec := reexecer.ReexecArgsForCall(0)
		Expect(reexecSpec.CloneUserns).To(BeFalse())
	})

	Context("when asked to clone user namespace", func() {
		var fakeReexecer *grootfakes.FakeSandboxReexecer

		BeforeEach(func() {
			fakeReexecer = new(grootfakes.FakeSandboxReexecer)
			reexecer = fakeReexecer
			cloneUsernsOnHandle = true
		})

		It("clones user namespace", func() {
			handler.Handle(logger, base_image_puller.UnpackSpec{TargetPath: childLayerPath, BaseDirectory: "/foo/bar"}, parentLayerPath)

			Expect(fakeReexecer.ReexecCallCount()).To(Equal(1))
			_, reexecSpec := fakeReexecer.ReexecArgsForCall(0)
			Expect(reexecSpec.CloneUserns).To(BeTrue())
		})
	})

	Context("when the base directory already exists in the child layer", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(childLayerPath, "foo"), 0755)).To(Succeed())
			Expect(ioutil.WriteFile(filepath.Join(childLayerPath, "foo", "a_file"), []byte{}, 0755))
			Expect(os.Chmod(filepath.Join(childLayerPath, "foo"), 0222)).To(Succeed())
			Expect(os.Chown(filepath.Join(childLayerPath, "foo"), 1000, 1000)).To(Succeed())
		})

		It("has no effect", func() {
			Expect(handler.Handle(logger, base_image_puller.UnpackSpec{TargetPath: childLayerPath, BaseDirectory: "/foo"}, parentLayerPath)).To(Succeed())
			Expect(filepath.Join(childLayerPath, "foo")).To(BeADirectory())
			Expect(filepath.Join(childLayerPath, "foo", "a_file")).To(BeAnExistingFile())
			Expect(getPermissions(filepath.Join(childLayerPath, "foo"))).To(Equal(os.FileMode(0222)))

			ownerUid, ownerGid := getOwner(filepath.Join(childLayerPath, "foo"))
			Expect(ownerUid).To(Equal(1000))
			Expect(ownerGid).To(Equal(1000))
		})
	})

	Context("when the base directory does not exist in the parent layer", func() {
		It("returns an error", func() {
			err := handler.Handle(logger, base_image_puller.UnpackSpec{TargetPath: childLayerPath, BaseDirectory: "/foo"}, parentLayerPath)
			Expect(err).To(MatchError(ContainSubstring("base directory not found in parent layer")))
		})
	})
})

func getOwner(path string) (int, int) {
	fileinfo, err := os.Stat(path)
	Expect(err).NotTo(HaveOccurred())
	sys := fileinfo.Sys().(*syscall.Stat_t)
	return int(sys.Uid), int(sys.Gid)
}

func getPermissions(path string) os.FileMode {
	fileinfo, err := os.Stat(path)
	Expect(err).NotTo(HaveOccurred())
	return fileinfo.Mode().Perm()
}

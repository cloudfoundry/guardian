package depot_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/guardian/rundmc/depot"
	"code.cloudfoundry.org/guardian/rundmc/depot/depotfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DepotBindMountSourceCreator", func() {
	var (
		chowner     *depotfakes.FakeChowner
		mountpoints []string

		containerDir string
		chown        bool
		creator      *depot.DepotBindMountSourceCreator

		bindMounts []garden.BindMount
		createErr  error
	)

	BeforeEach(func() {
		mountpoints = []string{"/etc/foo", "/etc/bar"}
		var err error
		containerDir, err = ioutil.TempDir("", "bundlerules-tests")
		Expect(err).NotTo(HaveOccurred())
		chown = true
		chowner = new(depotfakes.FakeChowner)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(containerDir)).To(Succeed())
	})

	JustBeforeEach(func() {
		creator = &depot.DepotBindMountSourceCreator{
			BindMountPoints:      mountpoints,
			ContainerRootHostUID: 875,
			ContainerRootHostGID: 876,
			Chowner:              chowner,
		}
		bindMounts, createErr = creator.Create(containerDir, chown)
	})

	It("returns no error", func() {
		Expect(createErr).NotTo(HaveOccurred())
	})

	It("creates empty source files for each mountpoint provided", func() {
		Expect(filepath.Join(containerDir, "foo")).To(BeARegularFile())
		Expect(filepath.Join(containerDir, "bar")).To(BeARegularFile())
	})

	It("chowns the created files to container root", func() {
		Expect(chowner.ChownCallCount()).To(Equal(2))
		path, uid, gid := chowner.ChownArgsForCall(0)
		Expect(path).To(Equal(filepath.Join(containerDir, "foo")))
		Expect(uid).To(Equal(875))
		Expect(gid).To(Equal(876))

		path, uid, gid = chowner.ChownArgsForCall(1)
		Expect(path).To(Equal(filepath.Join(containerDir, "bar")))
		Expect(uid).To(Equal(875))
		Expect(gid).To(Equal(876))
	})

	Context("when chown is not required", func() {
		BeforeEach(func() {
			chown = false
		})

		It("does not chown the created files", func() {
			Expect(chowner.ChownCallCount()).To(Equal(0))
		})
	})

	It("returns the bundle with the two mounts added", func() {
		Expect(bindMounts).To(Equal([]garden.BindMount{
			{
				DstPath: "/etc/foo",
				SrcPath: filepath.Join(containerDir, "foo"),
				Mode:    garden.BindMountModeRW,
			},
			{
				DstPath: "/etc/bar",
				SrcPath: filepath.Join(containerDir, "bar"),
				Mode:    garden.BindMountModeRW,
			},
		}))
	})

	Context("when chowning fails", func() {
		BeforeEach(func() {
			chowner.ChownReturns(errors.New("chown-error"))
		})

		It("returns an error", func() {
			Expect(createErr).To(HaveOccurred())
		})
	})

	Context("when two mountpoints have the same basename", func() {
		BeforeEach(func() {
			mountpoints = []string{"/etc/hosts", "/var/hosts"}
		})

		It("returns an error", func() {
			if runtime.GOOS == "windows" {
				Skip("Doesn't run on Windows")
			}
			Expect(createErr).To(HaveOccurred())
		})
	})
})

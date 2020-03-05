package preparerootfs_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	"github.com/docker/docker/pkg/reexec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func init() {
	if reexec.Init() {
		os.Exit(0)
	}
}

var _ = Describe("Preparerootfs", func() {
	var (
		rootfsPath string
		dir1, dir2 string
	)

	BeforeEach(func() {
		var err error
		rootfsPath, err = ioutil.TempDir("", "testdir")
		Expect(err).NotTo(HaveOccurred())

		dir1 = "potayto"
		dir2 = path.Join("poTARto", "banarna")

		SetDefaultEventuallyTimeout(5 * time.Second)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(rootfsPath)).To(Succeed())
	})

	run := func(rootfsPath string, uid, gid int, mode os.FileMode, recreate bool, args ...string) *gexec.Session {
		cmd := preparerootfs.Command(rootfsPath, uid, gid, mode, recreate, args...)
		sess, err := gexec.Start(cmd, gexec.NewPrefixedWriter("reexec-stdout: ", GinkgoWriter), gexec.NewPrefixedWriter("reexec-stderr: ", GinkgoWriter))
		Expect(err).NotTo(HaveOccurred())
		return sess
	}

	It("creates each directory", func() {
		Eventually(run(rootfsPath, 0, 0, 0755, true, dir1, dir2)).Should(gexec.Exit(0))

		Expect(path.Join(rootfsPath, dir1)).To(BeADirectory())
		Expect(path.Join(rootfsPath, dir2)).To(BeADirectory())
	})

	It("creates the directories as the requested uid and gid", func() {
		Eventually(run(rootfsPath, 12, 24, 0777, true, dir1, dir2)).Should(gexec.Exit(0))

		stat, err := os.Stat(path.Join(rootfsPath, dir2))
		Expect(err).NotTo(HaveOccurred())

		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(BeEquivalentTo(12))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(BeEquivalentTo(24))
	})

	It("chowns all created directories, not just the final directory", func() {
		Eventually(run(rootfsPath, 12, 24, 0700, true, dir1, dir2)).Should(gexec.Exit(0))

		stat, err := os.Stat(path.Join(rootfsPath, path.Dir(dir2)))
		Expect(err).NotTo(HaveOccurred())

		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(BeEquivalentTo(12))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(BeEquivalentTo(24))
	})

	It("sets the provided permissions in the directories", func() {
		Eventually(run(rootfsPath, 12, 24, 0700, true, dir1, dir2)).Should(gexec.Exit(0))

		stat, err := os.Stat(path.Join(rootfsPath, path.Dir(dir2)))
		Expect(err).NotTo(HaveOccurred())

		Expect(stat.Mode().Perm()).To(BeNumerically("==", 0700))
	})

	Context("when the rootfs contains a symlink", func() {
		It("is resolved relative to the rootfs, and not the host", func() {
			target, err := ioutil.TempDir("", "target")
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				Expect(os.RemoveAll(target)).To(Succeed())
			}()

			Expect(os.MkdirAll(path.Join(target, "foo", "shouldnotbedeleted"), 0700)).To(Succeed())
			Expect(os.Symlink(target, path.Join(rootfsPath, "test"))).To(Succeed())

			Eventually(run(rootfsPath, 0, 0, 0544, true, path.Join("test", "foo"))).Should(gexec.Exit(0))
			Expect(path.Join(target, "foo", "shouldnotbedeleted")).To(BeAnExistingFile())
			Expect(path.Join(rootfsPath, target)).To(BeADirectory()) // should've got created in rootfs
		})
	})

	Context("when the directory already exists", func() {
		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(rootfsPath, dir1), 0700)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(rootfsPath, dir1, "foo.txt"), []byte("brrr"), 0700)).To(Succeed())
		})

		It("does not fail", func() {
			Eventually(run(rootfsPath, 0, 0, 0755, true, dir1)).Should(gexec.Exit(0))
		})

		Context("when -recreate is specified", func() {
			It("removes any existing files from the directories", func() {
				Eventually(run(rootfsPath, 0, 0, 0744, true, dir1)).Should(gexec.Exit(0))
				Expect(path.Join(rootfsPath, dir1, "foo.txt")).NotTo(BeAnExistingFile())
			})
		})

		Context("when -recreate is NOT specified", func() {
			It("does not remove files from the existing directory", func() {
				Eventually(run(rootfsPath, 0, 0, 0744, false, dir1)).Should(gexec.Exit(0))
				Expect(path.Join(rootfsPath, dir1, "foo.txt")).To(BeAnExistingFile())
			})
		})
	})
})

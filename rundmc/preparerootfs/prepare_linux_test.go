package preparerootfs_test

import (
	"io/ioutil"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/cloudfoundry-incubator/guardian/rundmc/preparerootfs"
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

	run := func(rootfsPath string, uid, gid int, mode os.FileMode, args ...string) *gexec.Session {
		cmd := preparerootfs.Command(rootfsPath, uid, gid, mode, args...)
		sess, err := gexec.Start(cmd, gexec.NewPrefixedWriter("reexec-stdout: ", GinkgoWriter), gexec.NewPrefixedWriter("reexec-stderr: ", GinkgoWriter))
		Expect(err).NotTo(HaveOccurred())
		return sess
	}

	It("creates each directory", func() {
		Eventually(run(rootfsPath, 0, 0, 0755, dir1, dir2)).Should(gexec.Exit(0))

		Expect(path.Join(rootfsPath, dir1)).To(BeADirectory())
		Expect(path.Join(rootfsPath, dir2)).To(BeADirectory())
	})

	It("creates the directories as the requested uid and gid", func() {
		Eventually(run(rootfsPath, 12, 24, 0777, dir1, dir2)).Should(gexec.Exit(0))

		stat, err := os.Stat(path.Join(rootfsPath, dir2))
		Expect(err).NotTo(HaveOccurred())

		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(BeEquivalentTo(12))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(BeEquivalentTo(24))
	})

	It("chowns all created directories, not just the final directory", func() {
		Eventually(run(rootfsPath, 12, 24, 0700, dir1, dir2)).Should(gexec.Exit(0))

		stat, err := os.Stat(path.Join(rootfsPath, path.Dir(dir2)))
		Expect(err).NotTo(HaveOccurred())

		Expect(stat.Sys().(*syscall.Stat_t).Uid).To(BeEquivalentTo(12))
		Expect(stat.Sys().(*syscall.Stat_t).Gid).To(BeEquivalentTo(24))
	})

	It("sets the provided permissions in the directories", func() {
		Eventually(run(rootfsPath, 12, 24, 0700, dir1, dir2)).Should(gexec.Exit(0))

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

			Eventually(run(rootfsPath, 0, 0, 0544, path.Join("test", "foo"))).ShouldNot(gexec.Exit(0))
			Expect(path.Join(target, "foo", "shouldnotbedeleted")).To(BeAnExistingFile())
		})
	})

	Context("when the directory already exists", func() {
		It("does not fail", func() {
			Expect(os.MkdirAll(dir1, 0700)).To(Succeed())
			Eventually(run(rootfsPath, 0, 0, 0755, dir1)).Should(gexec.Exit(0))
		})

		It("removes any existing files from the directories", func() {
			Expect(os.MkdirAll(dir1, 0700)).To(Succeed())
			Expect(ioutil.WriteFile(path.Join(dir1, "foo.txt"), []byte("brrr"), 0700)).To(Succeed())

			Eventually(run(rootfsPath, 0, 0, 0744, dir1)).Should(gexec.Exit(0))

			Expect(path.Join(rootfsPath, dir1, "foo.txt")).NotTo(BeAnExistingFile())
		})
	})
})

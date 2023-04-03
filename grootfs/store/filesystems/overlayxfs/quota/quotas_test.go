package quota_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/store/filesystems/overlayxfs/quota"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Quotas", func() {
	var (
		directory     string
		xfsMountPoint string
		logger        *lagertest.TestLogger
	)

	BeforeEach(func() {
		var err error
		xfsMountPoint = XfsMountPointPool.Get().(string)
		Expect(xfsMountPoint).NotTo(BeEmpty())
		directory, err = ioutil.TempDir(xfsMountPoint, "images")
		Expect(err).NotTo(HaveOccurred())
		directory = filepath.Join(directory, "my-image")
		Expect(os.Mkdir(directory, 0755)).To(Succeed())

		logger = lagertest.NewTestLogger("quotas")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(filepath.Dir(directory))).To(Succeed())
	})

	Describe("Set", func() {
		It("enforces the quota on the path", func() {
			quota.Set(logger, 500, directory, 1024*1024)

			Eventually(writeFile(filepath.Join(directory, "small-file"), 500)).Should(gexec.Exit(0))

			sess := writeFile(filepath.Join(directory, "big-file"), 1000)
			Eventually(sess.Err).Should(gbytes.Say("No space left on device"))
			Eventually(sess).Should(gexec.Exit(1))
		})

		Context("when setting the quota to an unexisting path", func() {
			AfterEach(func() {
				XfsMountPointPool.Put(xfsMountPoint)
			})

			It("returns an error", func() {
				err := quota.Set(logger, 100, "/crazy-path", 1024)
				Expect(err).To(MatchError(ContainSubstring("opening directory: /crazy-path")))
			})
		})
	})

	Describe("Get", func() {
		BeforeEach(func() {
			quota.Set(logger, 500, directory, 10*1024*1024)
			Eventually(writeFile(filepath.Join(directory, "small-file"), 1024)).Should(gexec.Exit(0))
		})

		It("returns the quota with usage for the path", func() {
			quota, err := quota.Get(logger, directory)
			Expect(err).NotTo(HaveOccurred())
			Expect(quota.Size).To(Equal(uint64(10 * 1024 * 1024)))
			Expect(quota.BCount).To(Equal(uint64(1024 * 1024)))
		})

		Context("when the path doesn't have a quota applied", func() {
			var otherDir string

			BeforeEach(func() {
				var err error
				otherDir, err = ioutil.TempDir(xfsMountPoint, "images")
				Expect(err).NotTo(HaveOccurred())
				otherDir = filepath.Join(otherDir, "my-image")
				Expect(os.Mkdir(otherDir, 0755)).To(Succeed())
				Eventually(writeFile(filepath.Join(otherDir, "small-file"), 1024)).Should(gexec.Exit(0))
			})

			AfterEach(func() {
				Expect(os.RemoveAll(filepath.Dir(otherDir))).To(Succeed())
				XfsMountPointPool.Put(xfsMountPoint)
			})

			It("returns 0 usage", func() {
				quota, err := quota.Get(logger, otherDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(quota.Size).To(Equal(uint64(0)))
				Expect(quota.BCount).To(Equal(uint64(0)))
			})
		})

		Context("when getting the quota to an unexisting path", func() {
			AfterEach(func() {
				XfsMountPointPool.Put(xfsMountPoint)
			})

			It("returns an error", func() {
				_, err := quota.Get(logger, "/crazy-path")
				Expect(err).To(MatchError(ContainSubstring("opening directory: /crazy-path")))
			})
		})
	})

	Describe("GetProjectID", func() {
		BeforeEach(func() {
			quota.Set(logger, 1024, directory, 10*1024*1024)
			Eventually(writeFile(filepath.Join(directory, "small-file"), 1024)).Should(gexec.Exit(0))
		})

		It("returns the correct project id for the path", func() {
			projectID, err := quota.GetProjectID(logger, directory)
			Expect(err).NotTo(HaveOccurred())
			Expect(projectID).To(Equal(uint32(1024)))
		})

		Context("when getting the projectID from an unexisting path", func() {
			AfterEach(func() {
				XfsMountPointPool.Put(xfsMountPoint)
			})

			It("returns an error", func() {
				_, err := quota.GetProjectID(logger, "/crazy-path")
				Expect(err).To(MatchError(ContainSubstring("opening directory: /crazy-path")))
			})
		})

		Context("when the path doesn't have project id assigned", func() {
			var otherDir string

			BeforeEach(func() {
				var err error
				otherDir, err = ioutil.TempDir(xfsMountPoint, "images")
				Expect(err).NotTo(HaveOccurred())
				otherDir = filepath.Join(otherDir, "my-image")
				Expect(os.Mkdir(otherDir, 0755)).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(filepath.Dir(otherDir))).To(Succeed())
				XfsMountPointPool.Put(xfsMountPoint)
			})

			It("returns 0", func() {
				projectID, err := quota.GetProjectID(logger, otherDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(projectID).To(Equal(uint32(0)))
			})
		})
	})
})

func writeFile(path string, sizeKb int) *gexec.Session {
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%s", path), "bs=1K", fmt.Sprintf("count=%d", sizeKb))
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return sess
}

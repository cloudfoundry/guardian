package runrunc_test

import (
	"errors"
	"io/ioutil"
	"os"
	"path"
	"syscall"

	"github.com/cloudfoundry-incubator/guardian/rundmc/runrunc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("MkdirAs", func() {
	var (
		oldChownFunc func(string, int, int) error
		dirCreator   runrunc.DirectoryCreator
		tmpDirPath   string
	)

	BeforeEach(func() {
		oldChownFunc = runrunc.ChownFunc
		runrunc.ChownFunc = func(_ string, _, _ int) error {
			return nil
		}

		dirCreator = runrunc.DirectoryCreator{}

		var err error
		tmpDirPath, err = ioutil.TempDir("", "test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		runrunc.ChownFunc = oldChownFunc

		Expect(os.RemoveAll(tmpDirPath)).To(Succeed())
	})

	Context("when the path exists", func() {
		It("should succeed", func() {
			Expect(dirCreator.MkdirAs(tmpDirPath, 0755, 1012, 1013)).To(Succeed())
		})

		It("does not try to chown it", func() {
			chownFuncCallCount := 0
			runrunc.ChownFunc = func(filePath string, uid, gid int) error {
				chownFuncCallCount++
				return nil
			}

			Expect(dirCreator.MkdirAs(tmpDirPath, 0755, 1012, 1013)).To(Succeed())
			Expect(chownFuncCallCount).To(Equal(0))
		})
	})

	Context("when the directory doesn't already exist", func() {
		var (
			oldMask int

			nestedTmpDirPath string
		)

		BeforeEach(func() {
			oldMask = syscall.Umask(0022)

			nestedTmpDirPath = path.Join(tmpDirPath, "test/nest")
		})

		AfterEach(func() {
			syscall.Umask(oldMask)
		})

		It("creates the directory", func() {
			Expect(dirCreator.MkdirAs(nestedTmpDirPath, 0755, 1012, 1013)).To(Succeed())

			Expect(nestedTmpDirPath).To(BeADirectory())
		})

		It("creates the directory with the correct permissions", func() {
			Expect(dirCreator.MkdirAs(nestedTmpDirPath, 0755, 1012, 1013)).To(Succeed())

			dirStat, err := os.Stat(nestedTmpDirPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(dirStat.Mode().String()).To(Equal("drwxr-xr-x"))
		})

		It("sets the correct owner", func() {
			chownFuncCallCount := 0
			runrunc.ChownFunc = func(filePath string, uid, gid int) error {
				chownFuncCallCount++

				Expect(uid).To(Equal(1012))
				Expect(gid).To(Equal(1013))

				return nil
			}

			Expect(dirCreator.MkdirAs(nestedTmpDirPath, 0755, 1012, 1013)).To(Succeed())

			Expect(chownFuncCallCount).To(Equal(1))
		})

		Context("when setting the owner fails", func() {
			BeforeEach(func() {
				runrunc.ChownFunc = func(_ string, _ int, _ int) error {
					return errors.New("banana")
				}
			})

			It("returns the error", func() {
				Expect(
					dirCreator.MkdirAs(nestedTmpDirPath, 0755, 1012, 1013),
				).To(MatchError("banana"))
			})
		})
	})
})

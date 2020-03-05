package preparerootfs_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/guardian/rundmc/preparerootfs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("creating files without traversing symlinks", func() {
	var (
		dir       string
		creator   preparerootfs.SymlinkRefusingFileCreator
		createErr error
	)

	BeforeEach(func() {
		creator = preparerootfs.SymlinkRefusingFileCreator{}

		var err error
		dir, err = ioutil.TempDir("", "preparerootfs-test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(dir)).To(Succeed())
	})

	JustBeforeEach(func() {
		createErr = creator.CreateFiles(dir, "a", "b/c/d")
	})

	itCreatesTheFiles := func() {
		Expect(createErr).NotTo(HaveOccurred())
		Expect(filepath.Join(dir, "a")).To(BeARegularFile())
		Expect(filepath.Join(dir, "b", "c", "d")).To(BeARegularFile())
	}

	Context("when no structure exists in the dir", func() {
		It("creates the files", func() {
			itCreatesTheFiles()
		})
	})

	Context("when a file already exist in the dir", func() {
		BeforeEach(func() {
			Expect(ioutil.WriteFile(filepath.Join(dir, "a"), []byte("hi"), 0644)).To(Succeed())
		})

		It("creates the files", func() {
			itCreatesTheFiles()
		})

		It("preserves the contents of the files", func() {
			contents, err := ioutil.ReadFile(filepath.Join(dir, "a"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hi"))
		})
	})

	Context("when a file's parent directory exists, and is a symlink", func() {
		var linkedDir string

		BeforeEach(func() {
			var err error
			linkedDir, err = ioutil.TempDir("", "preparerootfs-test")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Symlink(linkedDir, filepath.Join(dir, "b"))).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(linkedDir)).To(Succeed())
		})

		It("returns an error", func() {
			Expect(createErr).To(HaveOccurred())
		})
	})
})

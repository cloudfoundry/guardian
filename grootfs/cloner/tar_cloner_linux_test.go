package cloner_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	grootpkg "code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TarCloner", func() {
	var (
		logger lager.Logger

		fromDir string
		toDir   string

		cloner *clonerpkg.TarCloner
	)

	BeforeEach(func() {
		var err error

		fromDir, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(fromDir, "a_file"), []byte("hello-world"), 0600)).To(Succeed())

		tempDir, err := ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())
		toDir = path.Join(tempDir, "rootfs")
	})

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test-graph")
		cloner = clonerpkg.NewTarCloner()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(fromDir)).To(Succeed())
		Expect(os.RemoveAll(toDir)).To(Succeed())
	})

	It("should have the image contents in the rootfs directory", func() {
		Expect(cloner.Clone(logger, grootpkg.CloneSpec{
			FromDir: fromDir,
			ToDir:   toDir,
		})).To(Succeed())

		filePath := path.Join(toDir, "a_file")
		Expect(filePath).To(BeARegularFile())
		contents, err := ioutil.ReadFile(filePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("hello-world"))
	})

	Context("when the image path does not exist", func() {
		It("should return an error", func() {
			Expect(cloner.Clone(logger, grootpkg.CloneSpec{
				FromDir: "/does/not/exist",
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring("image path `/does/not/exist` was not found")),
			)
		})
	})

	Context("when the image contains files that can only be read by root", func() {
		It("should return an error", func() {
			Expect(ioutil.WriteFile(path.Join(fromDir, "a-file"), []byte("hello-world"), 0000)).To(Succeed())

			Expect(cloner.Clone(logger, grootpkg.CloneSpec{
				FromDir: fromDir,
				ToDir:   toDir,
			})).To(
				MatchError(ContainSubstring(fmt.Sprintf("reading from `%s`", fromDir))),
			)
		})
	})

	Context("when untarring fails for reasons", func() {
		It("should return an error", func() {
			Expect(cloner.Clone(logger, grootpkg.CloneSpec{
				FromDir: fromDir,
				ToDir:   "/random/dir",
			})).To(
				MatchError(ContainSubstring(fmt.Sprintf("writing to `%s`", "/random/dir"))),
			)
		})
	})
})

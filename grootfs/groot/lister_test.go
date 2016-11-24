package groot_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lister", func() {
	var (
		storePath string
		logger    *lagertest.TestLogger
		lister    *groot.Lister
	)

	BeforeEach(func() {
		var err error
		storePath, err = ioutil.TempDir("", "")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(storePath, "root", "images", "a-root-image", "too-far"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(storePath, "groot", "images", "a-groot-image", "too-far"), 0755)).To(Succeed())
		logger = lagertest.NewTestLogger("iam-lister")

		lister = groot.IamLister()
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Describe("List", func() {
		It("lists images in store path", func() {
			var err error
			paths, err := lister.List(logger, storePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(paths)).To(Equal(2))
			Expect(paths).To(ContainElement(filepath.Join(storePath, "root", "images", "a-root-image")))
			Expect(paths).To(ContainElement(filepath.Join(storePath, "groot", "images", "a-groot-image")))
		})

		Context("when fails to list store path", func() {
			It("returns an error", func() {
				paths, err := lister.List(logger, "invalid-store-path")
				Expect(err).To(MatchError(ContainSubstring("failed to list store path")))
				Expect(paths).To(BeEmpty())
			})
		})

		Context("when fails to list sub store path", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(filepath.Join(storePath, "a-file"), []byte("a-file"), 0755)).To(Succeed())
			})

			It("returns an error", func() {
				paths, err := lister.List(logger, storePath)
				Expect(err).To(MatchError(ContainSubstring("failed to list substore path")))
				Expect(paths).To(BeEmpty())
			})
		})
	})
})

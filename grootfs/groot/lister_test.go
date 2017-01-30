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

		Expect(os.MkdirAll(filepath.Join(storePath, "images", "image-0", "too-far"), 0755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(storePath, "images", "image-1", "too-far"), 0755)).To(Succeed())
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
			Expect(paths).To(ContainElement(filepath.Join(storePath, "images", "image-0")))
			Expect(paths).To(ContainElement(filepath.Join(storePath, "images", "image-1")))
		})

		Context("when fails to list store path", func() {
			It("returns an error", func() {
				paths, err := lister.List(logger, "invalid-store-path")
				Expect(err).To(MatchError(ContainSubstring("failed to list store path")))
				Expect(paths).To(BeEmpty())
			})
		})
	})
})

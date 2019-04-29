package idfinder_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"code.cloudfoundry.org/grootfs/commands/idfinder"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Idfinder", func() {
	var (
		storePath string
		imageDir  string
		imageId   string
		err       error
	)

	BeforeEach(func() {
		imageId = "1234-my-id"
		storePath, err = ioutil.TempDir("", "")
		imageDir = filepath.Join(storePath, store.ImageDirName)
		Expect(os.MkdirAll(imageDir, 0777)).To(Succeed())
		Expect(err).NotTo(HaveOccurred())

		Expect(ioutil.WriteFile(path.Join(imageDir, imageId), []byte("hello-world"), 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(storePath)).To(Succeed())
	})

	Context("FindID", func() {
		Context("when a ID is provided", func() {
			It("returns the ID", func() {
				id, err := idfinder.FindID(storePath, imageId)
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal(imageId))
			})
		})

		Context("when a path is provided", func() {
			It("returns the ID", func() {
				id, err := idfinder.FindID(storePath, filepath.Join(imageDir, imageId))
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal(imageId))
			})

			Context("when the path is not within the store path", func() {
				It("returns an error", func() {
					_, err := idfinder.FindID(storePath, filepath.Join("/hello/not-store/path/images", imageId))
					Expect(err).To(MatchError("path `/hello/not-store/path/images/1234-my-id` is outside store path"))
				})
			})
		})

		It("returns an error when the image does not exist", func() {
			_, err := idfinder.FindID(storePath, filepath.Join(storePath, store.ImageDirName, "not-here"))
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("Image `not-here` not found")))
		})
	})
})

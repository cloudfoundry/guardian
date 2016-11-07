package idfinder_test

import (
	"code.cloudfoundry.org/grootfs/commands/idfinder"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Idfinder", func() {
	Context("FindID", func() {
		Context("when a ID is provided", func() {
			It("returns the ID", func() {
				id, err := idfinder.FindID("/hello/store/path", "1234-my-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("1234-my-id"))
			})
		})

		Context("when a path is provided", func() {
			It("returns the ID", func() {
				id, err := idfinder.FindID("/hello/store/path", "/hello/store/path/1200/images/1234-my-id")
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("1234-my-id"))
			})

			Context("when the path is not within the store path", func() {
				It("returns an error", func() {
					_, err := idfinder.FindID("/hello/store/path", "/hello/not-store/path/images/1234-my-id")
					Expect(err).To(MatchError("path `/hello/not-store/path/images/1234-my-id` is outside store path"))
				})
			})
		})
	})

	Context("SubStorePath", func() {
		It("returns the correct sub store path", func() {
			storePath, err := idfinder.FindSubStorePath("/hello/store/path", "/hello/store/path/1200/images/1234-my-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(storePath).To(Equal("/hello/store/path/1200"))
		})

		Context("when the path is not valid", func() {
			It("returns an error", func() {
				_, err := idfinder.FindSubStorePath("/hello/store/path", "/hello/store/path/images/1234-my-id")
				Expect(err).To(MatchError(ContainSubstring("unable to match substore in path `/hello/store/path/images/1234-my-id`")))
			})
		})
	})
})

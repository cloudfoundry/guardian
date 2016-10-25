package idfinder_test

import (
	"code.cloudfoundry.org/grootfs/commands/idfinder"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Idfinder", func() {
	Context("when a ID is provided", func() {
		It("returns the ID", func() {
			id, err := idfinder.FindID("/hello/store/path", "1234-my-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal("1234-my-id"))
		})
	})

	Context("when a path is provided", func() {
		It("returns the ID from the path", func() {
			id, err := idfinder.FindID("/hello/store/path", "/hello/store/path/bundles/1234-my-id")
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal("1234-my-id"))
		})

		Context("when the path is not within the store path", func() {
			It("returns an error", func() {
				_, err := idfinder.FindID("/hello/store/path", "/hello/not-store/path/bundles/1234-my-id")
				Expect(err).To(MatchError("path `/hello/not-store/path/bundles/1234-my-id` is outside store path"))
			})
		})
	})
})

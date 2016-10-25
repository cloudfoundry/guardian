package mapper_test

import (
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/idmapper/mapper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Subid", func() {
	Describe("LoadSubids", func() {
		var (
			subid         *os.File
			subidContents string
		)

		BeforeEach(func() {
			subidContents = "groot:100000:65536\nvcap:200000:65536"
		})

		JustBeforeEach(func() {
			var err error
			subid, err = ioutil.TempFile("", "subid")
			Expect(err).NotTo(HaveOccurred())
			defer subid.Close()

			_, err = subid.Write([]byte(subidContents))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(subid.Name())).To(Succeed())
		})

		It("correctly maps the subid file into a map", func() {
			subids, err := mapper.LoadSubids(subid.Name())
			Expect(err).NotTo(HaveOccurred())

			Expect(len(subids)).To(Equal(2))
			Expect(subids["groot"]).To(Equal(mapper.Subid{Start: 100000, Size: 65536}))
			Expect(subids["vcap"]).To(Equal(mapper.Subid{Start: 200000, Size: 65536}))
		})

		Context("when path does not exist", func() {
			It("returns an error", func() {
				_, err := mapper.LoadSubids("invalid-file-path")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when Start is invalid", func() {
			BeforeEach(func() {
				subidContents = "groot:foo:65536"
			})

			It("returns an error", func() {
				_, err := mapper.LoadSubids(subid.Name())
				Expect(err).To(MatchError(ContainSubstring("invalid start value")))
			})
		})

		Context("when Size is invalid", func() {
			BeforeEach(func() {
				subidContents = "groot:100000:foo"
			})

			It("returns an error", func() {
				_, err := mapper.LoadSubids(subid.Name())
				Expect(err).To(MatchError(ContainSubstring("invalid size value")))
			})
		})
	})
})

package remote_test

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"os"

	"code.cloudfoundry.org/grootfs/fetcher/remote"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BlobReader", func() {
	var (
		blobReader *remote.BlobReader
		blobFile   *os.File
	)

	Context("when the blob exists", func() {
		BeforeEach(func() {
			gzipBuffer := bytes.NewBuffer([]byte{})
			gzipWriter := gzip.NewWriter(gzipBuffer)
			gzipWriter.Write([]byte("hello-world"))
			gzipWriter.Close()
			gzipedBlobContent, err := ioutil.ReadAll(gzipBuffer)

			blobFile, err = ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
			_, err = blobFile.Write(gzipedBlobContent)
			Expect(err).NotTo(HaveOccurred())
			blobReader, err = remote.NewBlobReader(blobFile.Name())
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Read", func() {
			It("reads the gziped stream", func() {
				contents, err := ioutil.ReadAll(blobReader)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("hello-world"))
			})

		})

		Describe("Close", func() {
			It("deletes the source blob file", func() {
				Expect(blobFile.Name()).To(BeAnExistingFile())
				Expect(blobReader.Close()).To(Succeed())
				Expect(blobFile.Name()).ToNot(BeAnExistingFile())
			})
		})
	})

	Context("when the blob is not gziped", func() {
		BeforeEach(func() {
			err := ioutil.WriteFile(blobFile.Name(), []byte("im-not-gziped!"), 0700)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("NewBlobReader", func() {
			It("returns an error", func() {
				_, err := remote.NewBlobReader(blobFile.Name())
				Expect(err).To(MatchError(ContainSubstring("blob file is not gzipped")))
			})
		})
	})

	Context("when the blob doesn't exist", func() {
		Describe("NewBlobReader", func() {
			It("returns an error", func() {
				_, err := remote.NewBlobReader("not-a-real/file")
				Expect(err).To(MatchError(ContainSubstring("failed to open blob")))
			})
		})
	})
})

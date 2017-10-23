package layer_fetcher_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BlobReader", func() {
	var (
		blobReader       *layer_fetcher.BlobReader
		blobFile         *os.File
		newBlobReaderErr error
	)

	BeforeEach(func() {
		gzipBuffer := bytes.NewBuffer([]byte{})
		gzipWriter := gzip.NewWriter(gzipBuffer)
		writeString(gzipWriter, "hello-world")
		Expect(gzipWriter.Close()).To(Succeed())

		blobFile = tempFile()
		defer blobFile.Close()
		writeString(blobFile, readAll(gzipBuffer))
	})

	AfterEach(func() {
		removeAllIfTemp(blobFile.Name())
	})

	JustBeforeEach(func() {
		blobReader, newBlobReaderErr = layer_fetcher.NewBlobReader(blobFile.Name(), "")
		Expect(newBlobReaderErr).NotTo(HaveOccurred())
	})

	Describe("Read", func() {
		It("reads the gziped stream", func() {
			Expect(readAll(blobReader)).To(Equal("hello-world"))
		})

		Context("and the MediaType is explicitly gzip", func() {
			JustBeforeEach(func() {
				blobReader, newBlobReaderErr = layer_fetcher.NewBlobReader(blobFile.Name(), "application/vnd.oci.image.layer.v1.tar+gzip")
				Expect(newBlobReaderErr).NotTo(HaveOccurred())
			})

			It("reads the gziped stream", func() {
				Expect(readAll(blobReader)).To(Equal("hello-world"))
			})
		})

		Context("when the blob is not gziped", func() {
			var notABlobFile *os.File
			BeforeEach(func() {
				var err error
				notABlobFile = tempFile()
				defer notABlobFile.Close()
				err = ioutil.WriteFile(notABlobFile.Name(), []byte("im-not-gziped!"), 0700)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				removeAllIfTemp(notABlobFile.Name())
			})

			Describe("NewBlobReader", func() {
				It("does not return an error", func() {
					blobReader, err := layer_fetcher.NewBlobReader(notABlobFile.Name(), "application/vnd.oci.image.layer.v1.tar")
					Expect(err).NotTo(HaveOccurred())
					Expect(readAll(blobReader)).To(Equal("im-not-gziped!"))
				})
			})
		})
	})

	Context("when the blob doesn't exist", func() {
		Describe("NewBlobReader", func() {
			It("returns an error", func() {
				_, err := layer_fetcher.NewBlobReader("not-a-real/file", "")
				Expect(err).To(MatchError(ContainSubstring("failed to open blob")))
			})
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

func readAll(reader io.Reader) string {
	contents, err := ioutil.ReadAll(reader)
	Expect(err).NotTo(HaveOccurred())
	return string(contents)
}

func writeString(writer io.Writer, contents string) {
	size, err := io.WriteString(writer, contents)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(contents)).To(Equal(size))
}

func tempFile() *os.File {
	file, err := ioutil.TempFile("", "")
	Expect(err).NotTo(HaveOccurred())
	return file
}

func removeAllIfTemp(path string) {
	if !strings.HasPrefix(path, os.TempDir()) {
		Fail("attempt to delete non-temp file: " + path)
	}

	os.RemoveAll(path)
	Expect(path).NotTo(BeAnExistingFile())
}

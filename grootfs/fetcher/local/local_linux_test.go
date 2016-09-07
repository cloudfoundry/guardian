package local_test

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/fetcher/local"
	"code.cloudfoundry.org/grootfs/image_puller"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	. "github.com/st3v/glager"
)

var _ = Describe("Local Fetcher", func() {
	var (
		fetcher *local.LocalFetcher

		imagePath string
		logger    *TestLogger
		imageURL  *url.URL
	)

	BeforeEach(func() {
		fetcher = local.NewLocalFetcher()

		var err error
		imagePath, err = ioutil.TempDir("", "image")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(imagePath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())

		imageURL, err = url.Parse(imagePath)
		Expect(err).NotTo(HaveOccurred())

		logger = NewLogger("local-fetcher")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	Describe("StreamBlob", func() {
		It("returns the contents of the source directory as a Tar stream", func() {
			stream, _, err := fetcher.StreamBlob(logger, imageURL, "")
			Expect(err).ToNot(HaveOccurred())

			entries := streamTar(tar.NewReader(stream))
			Expect(entries).To(HaveLen(2))
			Expect(entries[1].header.Name).To(Equal("./a_file"))
			Expect(entries[1].header.Mode).To(Equal(int64(0600)))
			Expect(string(entries[1].contents)).To(Equal("hello-world"))
		})

		It("logs the tar command", func() {
			_, _, err := fetcher.StreamBlob(logger, imageURL, "")
			Expect(err).ToNot(HaveOccurred())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("local-fetcher.stream-blob.starting-tar"),
					Data("args", []string{"tar", "-cp", "-C", imagePath, "."}),
				),
			))
		})

		Context("when the source does not exist", func() {
			It("returns an error", func() {
				nonExistentImageURL, _ := url.Parse("/nothing/here")

				_, _, err := fetcher.StreamBlob(logger, nonExistentImageURL, "")
				Expect(err).To(MatchError(ContainSubstring("local image not found in `/nothing/here`")))
			})
		})

		Context("when tar does not exist", func() {
			It("returns an error", func() {
				local.TarBin = "non-existent-tar"

				_, _, err := fetcher.StreamBlob(logger, imageURL, "")
				Expect(err).To(MatchError(ContainSubstring("reading local image")))
			})
		})
	})

	Describe("LayersDigest", func() {
		var imageInfo image_puller.ImageInfo

		BeforeEach(func() {
			var err error
			imageInfo, err = fetcher.ImageInfo(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the correct image", func() {
			layers := imageInfo.LayersDigest

			Expect(len(layers)).To(Equal(1))
			Expect(layers[0].BlobID).To(Equal(imagePath))
			Expect(layers[0].DiffID).To(BeEmpty())
			Expect(layers[0].ChainID).NotTo(BeEmpty())
			Expect(layers[0].ParentChainID).To(BeEmpty())

			Expect(imageInfo.Config).To(Equal(specsv1.Image{}))
		})

		Context("when image content gets updated", func() {
			BeforeEach(func() {
				time.Sleep(time.Millisecond * 10)
				Expect(ioutil.WriteFile(filepath.Join(imagePath, "foobar"), []byte("hello-world"), 0700)).To(Succeed())
			})

			It("generates another volume id", func() {
				newImageInfo, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(imageInfo.LayersDigest[0].ChainID).NotTo(Equal(newImageInfo.LayersDigest[0].ChainID))
			})
		})

		Context("when the image doesn't exist", func() {
			BeforeEach(func() {
				var err error
				imageURL, err = url.Parse("/not-here")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("fetching image timestamp")))
			})
		})
	})
})

type tarEntry struct {
	header   *tar.Header
	contents []byte
}

func streamTar(r *tar.Reader) []tarEntry {
	l := []tarEntry{}
	for {
		header, err := r.Next()
		if err != nil {
			Expect(err).To(Equal(io.EOF))
			return l
		}

		contents := make([]byte, header.Size)
		r.Read(contents)
		l = append(l, tarEntry{
			header:   header,
			contents: contents,
		})
	}
}

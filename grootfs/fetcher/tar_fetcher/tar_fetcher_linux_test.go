package tar_fetcher_test

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"
	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher/tar_fetcher"
	"code.cloudfoundry.org/grootfs/integration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
	. "github.com/st3v/glager"
)

var _ = Describe("Tar Fetcher", func() {
	var (
		fetcher *fetcherpkg.TarFetcher

		sourceImagePath string
		baseImagePath   string
		logger          *TestLogger
		baseImageURL    *url.URL
	)

	BeforeEach(func() {
		fetcher = fetcherpkg.NewTarFetcher()

		var err error
		sourceImagePath, err = ioutil.TempDir("", "image")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())
		logger = NewLogger("tar-fetcher")
	})

	JustBeforeEach(func() {
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
		var err error
		baseImageURL, err = url.Parse(baseImagePath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
	})

	Describe("StreamBlob", func() {
		It("returns the contents of the source directory as a Tar stream", func() {
			stream, _, err := fetcher.StreamBlob(logger, baseImageURL, "")
			Expect(err).ToNot(HaveOccurred())

			entries := streamTar(tar.NewReader(stream))
			Expect(entries).To(HaveLen(2))
			Expect(entries[1].header.Name).To(Equal("./a_file"))
			Expect(entries[1].header.Mode).To(Equal(int64(0600)))
			Expect(string(entries[1].contents)).To(Equal("hello-world"))
		})

		It("logs the tar command", func() {
			_, _, err := fetcher.StreamBlob(logger, baseImageURL, "")
			Expect(err).ToNot(HaveOccurred())

			Expect(logger).To(ContainSequence(
				Debug(
					Message("tar-fetcher.stream-blob.opening-tar"),
					Data("baseImagePath", baseImagePath),
				),
			))
		})

		Context("when the source is a directory", func() {
			It("returns an error message", func() {
				tempDir, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				imageURL, _ := url.Parse(tempDir)
				_, _, err = fetcher.StreamBlob(logger, imageURL, "")
				Expect(err).To(MatchError(ContainSubstring("invalid base image: directory provided instead of a tar file")))
			})
		})

		Context("when the source does not exist", func() {
			It("returns an error", func() {
				nonExistentImageURL, _ := url.Parse("/nothing/here")

				_, _, err := fetcher.StreamBlob(logger, nonExistentImageURL, "")
				Expect(err).To(MatchError(ContainSubstring("local image not found in `/nothing/here`")))
			})
		})
	})

	Describe("LayersDigest", func() {
		var baseImageInfo base_image_puller.BaseImageInfo

		JustBeforeEach(func() {
			var err error
			baseImageInfo, err = fetcher.BaseImageInfo(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the correct image", func() {
			layers := baseImageInfo.LayersDigest

			Expect(len(layers)).To(Equal(1))
			Expect(layers[0].BlobID).To(Equal(baseImagePath))
			Expect(layers[0].ChainID).NotTo(BeEmpty())
			Expect(layers[0].ParentChainID).To(BeEmpty())

			Expect(baseImageInfo.Config).To(Equal(specsv1.Image{}))
		})

		Context("when image content gets updated", func() {
			JustBeforeEach(func() {
				time.Sleep(time.Millisecond * 10)
				Expect(ioutil.WriteFile(filepath.Join(sourceImagePath, "foobar"), []byte("hello-world"), 0700)).To(Succeed())
				integration.UpdateBaseImageTar(baseImagePath, sourceImagePath)
			})

			It("generates another volume id", func() {
				newBaseImageInfo, err := fetcher.BaseImageInfo(logger, baseImageURL)
				Expect(err).NotTo(HaveOccurred())
				Expect(baseImageInfo.LayersDigest[0].ChainID).NotTo(Equal(newBaseImageInfo.LayersDigest[0].ChainID))
			})
		})

		Context("when the image doesn't exist", func() {
			JustBeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("/not-here")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := fetcher.BaseImageInfo(logger, baseImageURL)
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
		_, _ = r.Read(contents)
		l = append(l, tarEntry{
			header:   header,
			contents: contents,
		})
	}
}

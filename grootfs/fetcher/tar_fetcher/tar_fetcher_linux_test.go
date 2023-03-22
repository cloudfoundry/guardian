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

	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher/tar_fetcher"
	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/integration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/st3v/glager"
)

var _ = Describe("Tar Fetcher", func() {
	var (
		fetcher *fetcherpkg.TarFetcher

		sourceImagePath string
		baseImagePath   string
		logger          *glager.TestLogger
		baseImageURL    *url.URL
	)

	BeforeEach(func() {

		var err error
		sourceImagePath, err = ioutil.TempDir("", "image")
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(path.Join(sourceImagePath, "a_file"), []byte("hello-world"), 0600)).To(Succeed())
		logger = glager.NewLogger("tar-fetcher")
		baseImageFile := integration.CreateBaseImageTar(sourceImagePath)
		baseImagePath = baseImageFile.Name()
		baseImageURL, err = url.Parse(baseImagePath)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		fetcher = fetcherpkg.NewTarFetcher(baseImageURL)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(baseImagePath)).To(Succeed())
		Expect(os.RemoveAll(sourceImagePath)).To(Succeed())
	})

	Describe("StreamBlob", func() {
		It("returns the contents of the source directory as a Tar stream", func() {
			stream, _, err := fetcher.StreamBlob(logger, groot.LayerInfo{})
			Expect(err).ToNot(HaveOccurred())

			entries := streamTar(tar.NewReader(stream))
			Expect(entries).To(HaveLen(2))
			Expect(entries[1].header.Name).To(Equal("./a_file"))
			Expect(entries[1].header.Mode).To(Equal(int64(0600)))
			Expect(string(entries[1].contents)).To(Equal("hello-world"))
		})

		It("logs the tar command", func() {
			_, _, err := fetcher.StreamBlob(logger, groot.LayerInfo{})
			Expect(err).ToNot(HaveOccurred())

			Expect(logger).To(glager.ContainSequence(
				glager.Debug(
					glager.Message("tar-fetcher.stream-blob.opening-tar"),
					glager.Data("baseImagePath", baseImagePath),
				),
			))
		})

		Context("when the source is a directory", func() {
			BeforeEach(func() {
				tempDir, err := ioutil.TempDir("", "")
				Expect(err).NotTo(HaveOccurred())

				baseImageURL, _ = url.Parse(tempDir)
			})
			It("returns an error message", func() {
				_, _, err := fetcher.StreamBlob(logger, groot.LayerInfo{})
				Expect(err).To(MatchError(ContainSubstring("invalid base image: directory provided instead of a tar file")))
			})
		})

		Context("when the source does not exist", func() {
			BeforeEach(func() {
				baseImageURL, _ = url.Parse("/nothing/here")
			})

			It("returns an error", func() {

				_, _, err := fetcher.StreamBlob(logger, groot.LayerInfo{})
				Expect(err).To(MatchError(ContainSubstring("local image not found in `/nothing/here`")))
			})
		})
	})

	Describe("LayersDigest", func() {
		var (
			baseImageInfo groot.BaseImageInfo
			imageInfoErr  error
		)

		JustBeforeEach(func() {
			baseImageInfo, imageInfoErr = fetcher.BaseImageInfo(logger)
		})

		It("returns the correct image", func() {
			layers := baseImageInfo.LayerInfos

			Expect(len(layers)).To(Equal(1))
			Expect(layers[0].BlobID).To(Equal(baseImagePath))
			Expect(layers[0].ChainID).NotTo(BeEmpty())
			Expect(layers[0].ParentChainID).To(BeEmpty())

			Expect(baseImageInfo.Config).To(Equal(v1.Image{}))
		})

		Context("when image content gets updated", func() {
			JustBeforeEach(func() {
				time.Sleep(time.Millisecond * 10)
				Expect(ioutil.WriteFile(filepath.Join(sourceImagePath, "foobar"), []byte("hello-world"), 0700)).To(Succeed())
				integration.UpdateBaseImageTar(baseImagePath, sourceImagePath)
			})

			It("generates another volume id", func() {
				newBaseImageInfo, err := fetcher.BaseImageInfo(logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(baseImageInfo.LayerInfos[0].ChainID).NotTo(Equal(newBaseImageInfo.LayerInfos[0].ChainID))
			})
		})

		Context("when the image doesn't exist", func() {
			BeforeEach(func() {
				var err error
				baseImageURL, err = url.Parse("/not-here")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error", func() {
				Expect(imageInfoErr).To(MatchError(ContainSubstring("fetching image timestamp")))
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

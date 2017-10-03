package layer_fetcher_test

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io/ioutil"
	"net/url"
	"time"

	"code.cloudfoundry.org/grootfs/base_image_puller"

	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/layer_fetcher/layer_fetcherfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	digestpkg "github.com/opencontainers/go-digest"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("LayerFetcher", func() {
	var (
		fakeSource        *layer_fetcherfakes.FakeSource
		fetcher           *layer_fetcher.LayerFetcher
		logger            *lagertest.TestLogger
		baseImageURL      *url.URL
		gzipedBlobContent []byte
	)

	BeforeEach(func() {
		fakeSource = new(layer_fetcherfakes.FakeSource)

		gzipBuffer := bytes.NewBuffer([]byte{})
		gzipWriter := gzip.NewWriter(gzipBuffer)
		_, err := gzipWriter.Write([]byte("hello-world"))
		Expect(err).NotTo(HaveOccurred())
		Expect(gzipWriter.Close()).To(Succeed())
		gzipedBlobContent, err = ioutil.ReadAll(gzipBuffer)
		Expect(err).NotTo(HaveOccurred())

		fetcher = layer_fetcher.NewLayerFetcher(fakeSource)

		logger = lagertest.NewTestLogger("test-layer-fetcher")
		baseImageURL, err = url.Parse("docker:///cfgarden/empty:v0.1.1")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("BaseImageInfo", func() {
		It("fetches the manifest", func() {
			fakeManifest := new(layer_fetcherfakes.FakeManifest)
			fakeManifest.OCIConfigReturns(&specsv1.Image{}, nil)
			fakeSource.ManifestReturns(fakeManifest, nil)

			_, err := fetcher.BaseImageInfo(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeSource.ManifestCallCount()).To(Equal(1))
			_, usedImageURL := fakeSource.ManifestArgsForCall(0)
			Expect(usedImageURL).To(Equal(baseImageURL))
		})

		It("closes the manifest", func() {
			fakeManifest := new(layer_fetcherfakes.FakeManifest)
			fakeManifest.OCIConfigReturns(&specsv1.Image{}, nil)
			fakeSource.ManifestReturns(fakeManifest, nil)

			_, err := fetcher.BaseImageInfo(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeManifest.CloseCallCount()).To(Equal(1))
		})

		Context("when fetching the manifest fails", func() {
			BeforeEach(func() {
				fakeSource.ManifestReturns(nil, errors.New("fetching the manifest"))
			})

			It("returns an error", func() {
				_, err := fetcher.BaseImageInfo(logger, baseImageURL)
				Expect(err).To(MatchError(ContainSubstring("fetching the manifest")))
			})
		})

		It("returns the correct list of layer digests", func() {
			config := &specsv1.Image{
				RootFS: specsv1.RootFS{
					DiffIDs: []digestpkg.Digest{
						digestpkg.NewDigestFromHex("sha256", "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5"),
						digestpkg.NewDigestFromHex("sha256", "d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0"),
					},
				},
			}
			fakeManifest := new(layer_fetcherfakes.FakeManifest)
			fakeManifest.OCIConfigReturns(config, nil)
			fakeManifest.LayerInfosReturns([]types.BlobInfo{
				types.BlobInfo{
					Digest:      digestpkg.NewDigestFromHex("sha256", "47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58"),
					Size:        1024,
					Annotations: map[string]string{"org.cloudfoundry.experimental.image.base-directory": "/home/cool-user"},
				},
				types.BlobInfo{
					Digest: digestpkg.NewDigestFromHex("sha256", "7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76"),
					Size:   2048,
				},
			})
			fakeSource.ManifestReturns(fakeManifest, nil)

			baseImageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.1")
			Expect(err).NotTo(HaveOccurred())

			baseImageInfo, err := fetcher.BaseImageInfo(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(baseImageInfo.LayerInfos).To(Equal([]base_image_puller.LayerInfo{
				base_image_puller.LayerInfo{
					BlobID:        "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
					ChainID:       "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
					ParentChainID: "",
					BaseDirectory: "/home/cool-user",
					Size:          1024,
				},
				base_image_puller.LayerInfo{
					BlobID:        "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
					ChainID:       "9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233",
					ParentChainID: "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
					Size:          2048,
				},
			}))
		})

		Context("when retrieving the OCI Config fails", func() {
			BeforeEach(func() {
				fakeManifest := new(layer_fetcherfakes.FakeManifest)
				fakeManifest.OCIConfigReturns(&specsv1.Image{}, errors.New("OCI Config retrieval failed"))
				fakeSource.ManifestReturns(fakeManifest, nil)
			})

			It("returns the error", func() {
				_, err := fetcher.BaseImageInfo(logger, baseImageURL)
				Expect(err).To(MatchError(ContainSubstring("OCI Config retrieval failed")))
			})
		})

		It("returns the correct OCI image config", func() {
			timestamp := time.Time{}.In(time.UTC)
			expectedConfig := specsv1.Image{
				Created: &timestamp,
				RootFS: specsv1.RootFS{
					DiffIDs: []digestpkg.Digest{
						digestpkg.NewDigestFromHex("sha256", "afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5"),
						digestpkg.NewDigestFromHex("sha256", "d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0"),
					},
				},
			}

			fakeManifest := new(layer_fetcherfakes.FakeManifest)
			fakeManifest.OCIConfigReturns(&expectedConfig, nil)
			fakeSource.ManifestReturns(fakeManifest, nil)

			baseImageInfo, err := fetcher.BaseImageInfo(logger, baseImageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(baseImageInfo.Config).To(Equal(expectedConfig))
		})
	})

	Describe("StreamBlob", func() {
		var layerInfo = base_image_puller.LayerInfo{
			BlobID: "sha256:layer-digest",
		}
		BeforeEach(func() {
			tmpFile, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
			_, err = tmpFile.Write(gzipedBlobContent)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = tmpFile.Close() }()

			fakeSource.BlobReturns(tmpFile.Name(), 0, nil)
		})

		It("uses the source", func() {
			_, _, err := fetcher.StreamBlob(logger, baseImageURL, layerInfo)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeSource.BlobCallCount()).To(Equal(1))
			_, usedImageURL, usedDigest, _ := fakeSource.BlobArgsForCall(0)
			Expect(usedImageURL).To(Equal(baseImageURL))
			Expect(usedDigest).To(Equal("sha256:layer-digest"))
		})

		It("returns the stream from the source", func(done Done) {
			stream, _, err := fetcher.StreamBlob(logger, baseImageURL, layerInfo)
			Expect(err).NotTo(HaveOccurred())

			contents, err := ioutil.ReadAll(stream)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))

			close(done)
		}, 2.0)

		It("returns the size of the stream", func() {
			tmpFile, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = tmpFile.Close() }()

			gzipWriter := gzip.NewWriter(tmpFile)
			Expect(gzipWriter.Close()).To(Succeed())

			fakeSource.BlobReturns(tmpFile.Name(), 1024, nil)

			_, size, err := fetcher.StreamBlob(logger, baseImageURL, layerInfo)
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(1024)))
		})

		Context("when the source fails to stream the blob", func() {
			It("returns an error", func() {
				fakeSource.BlobReturns("", 0, errors.New("failed to stream blob"))

				_, _, err := fetcher.StreamBlob(logger, baseImageURL, layerInfo)
				Expect(err).To(MatchError(ContainSubstring("failed to stream blob")))
			})
		})
	})
})

package remote_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/url"
	"os"

	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/fetcherfakes"
	"code.cloudfoundry.org/grootfs/fetcher/remote"
	"code.cloudfoundry.org/grootfs/fetcher/remote/remotefakes"
	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("RemoteFetcher", func() {
	var (
		fakeCacheDriver   *fetcherfakes.FakeCacheDriver
		fakeSource        *remotefakes.FakeSource
		fetcher           *remote.RemoteFetcher
		logger            *lagertest.TestLogger
		imageURL          *url.URL
		gzipedBlobContent []byte
	)

	BeforeEach(func() {
		var err error
		fakeSource = new(remotefakes.FakeSource)
		fakeCacheDriver = new(fetcherfakes.FakeCacheDriver)

		gzipBuffer := bytes.NewBuffer([]byte{})
		gzipWriter := gzip.NewWriter(gzipBuffer)
		gzipWriter.Write([]byte("hello-world"))
		gzipWriter.Close()
		gzipedBlobContent, err = ioutil.ReadAll(gzipBuffer)
		Expect(err).NotTo(HaveOccurred())

		// by default, the cache driver does not do any caching
		fakeCacheDriver.StreamBlobStub = func(logger lager.Logger, id string,
			remoteBlobFunc fetcherpkg.RemoteBlobFunc,
		) (io.ReadCloser, int64, error) {
			contents, size, err := remoteBlobFunc(logger)
			if err != nil {
				return nil, 0, err
			}

			return ioutil.NopCloser(bytes.NewBuffer(contents)), size, nil
		}

		fetcher = remote.NewRemoteFetcher(fakeSource, fakeCacheDriver)

		logger = lagertest.NewTestLogger("test-remote-fetcher")
		imageURL, err = url.Parse("docker:///cfgarden/empty:v0.1.1")
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("ImageInfo", func() {
		It("fetches the manifest", func() {
			_, err := fetcher.ImageInfo(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeSource.ManifestCallCount()).To(Equal(1))
			_, usedImageURL := fakeSource.ManifestArgsForCall(0)
			Expect(usedImageURL).To(Equal(imageURL))
		})

		Context("when fetching the manifest fails", func() {
			BeforeEach(func() {
				fakeSource.ManifestReturns(remote.Manifest{}, errors.New("fetching the manifest"))
			})

			It("returns an error", func() {
				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("fetching the manifest")))
			})
		})

		It("returns the correct list of layer digests", func() {
			manifest := remote.Manifest{
				Layers: []string{
					"sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
					"sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
				},
			}
			fakeSource.ManifestReturns(manifest, nil)
			fakeSource.ConfigReturns(specsv1.Image{
				RootFS: specsv1.RootFS{
					DiffIDs: []string{
						"sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
						"sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
					},
				},
			}, nil)

			imageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.1")
			Expect(err).NotTo(HaveOccurred())

			imageInfo, err := fetcher.ImageInfo(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(imageInfo.LayersDigest).To(Equal([]image_puller.LayerDigest{
				image_puller.LayerDigest{
					BlobID:        "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
					ChainID:       "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
					ParentChainID: "",
				},
				image_puller.LayerDigest{
					BlobID:        "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
					ChainID:       "sha256:9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233",
					ParentChainID: "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
				},
			}))
		})

		It("calls the source", func() {
			manifest := remote.Manifest{
				ConfigCacheKey: "sha256:hello-world",
			}
			fakeSource.ManifestReturns(manifest, nil)

			_, err := fetcher.ImageInfo(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeSource.ConfigCallCount()).To(Equal(1))
			_, usedImageURL, usedManifest := fakeSource.ConfigArgsForCall(0)
			Expect(usedImageURL).To(Equal(imageURL))
			Expect(usedManifest).To(Equal(manifest))
		})

		Context("when fetching the config fails", func() {
			BeforeEach(func() {
				fakeSource.ConfigReturns(specsv1.Image{}, errors.New("fetching the config"))
			})

			It("returns the error", func() {
				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("fetching the config")))
			})
		})

		It("returns the correct image config", func() {
			expectedConfig := specsv1.Image{
				RootFS: specsv1.RootFS{
					DiffIDs: []string{
						"sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
						"sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
					},
				},
			}
			fakeSource.ConfigReturns(expectedConfig, nil)

			imageInfo, err := fetcher.ImageInfo(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(imageInfo.Config).To(Equal(expectedConfig))
		})

		Context("when the config is in the cache", func() {
			var (
				configWriter   io.WriteCloser
				expectedConfig specsv1.Image
				configContents []byte
			)

			BeforeEach(func() {
				var (
					configReader io.ReadCloser
					err          error
				)

				configReader, configWriter, err = os.Pipe()
				Expect(err).NotTo(HaveOccurred())

				fakeCacheDriver.StreamBlobReturns(configReader, 0, nil)

				expectedConfig = specsv1.Image{
					RootFS: specsv1.RootFS{
						DiffIDs: []string{
							"sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
							"sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
						},
					},
				}
				configContents, err = json.Marshal(expectedConfig)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				_, err := configWriter.Write(configContents)
				Expect(err).NotTo(HaveOccurred())

				Expect(configWriter.Close()).To(Succeed())
			})

			It("calls the cache driver", func() {
				manifest := remote.Manifest{
					ConfigCacheKey: "sha256:cached-config",
				}
				fakeSource.ManifestReturns(manifest, nil)

				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeCacheDriver.StreamBlobCallCount()).To(Equal(1))
				_, id, _ := fakeCacheDriver.StreamBlobArgsForCall(0)
				Expect(id).To(Equal("sha256:cached-config"))
			})

			It("returns the correct image config", func() {
				imageInfo, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(imageInfo.Config).To(Equal(expectedConfig))
			})

			Context("when the cache returns a corrupted config", func() {
				BeforeEach(func() {
					_, err := configWriter.Write([]byte("{invalid: json"))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns an error", func() {
					_, err := fetcher.ImageInfo(logger, imageURL)
					Expect(err).To(MatchError(ContainSubstring("decoding config from JSON")))
				})
			})
		})

		Context("when the cache fails", func() {
			It("returns the error", func() {
				fakeCacheDriver.StreamBlobReturns(nil, 0, errors.New("failed to return"))

				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("failed to return")))
			})
		})
	})

	Describe("StreamBlob", func() {
		BeforeEach(func() {
			fakeSource.BlobReturns(gzipedBlobContent, 0, nil)
		})

		It("uses the source", func() {
			_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeSource.BlobCallCount()).To(Equal(1))
			_, usedImageURL, usedDigest := fakeSource.BlobArgsForCall(0)
			Expect(usedImageURL).To(Equal(imageURL))
			Expect(usedDigest).To(Equal("sha256:layer-digest"))
		})

		It("returns the stream from the source", func(done Done) {
			stream, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
			Expect(err).NotTo(HaveOccurred())

			contents, err := ioutil.ReadAll(stream)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))

			close(done)
		}, 2.0)

		It("returns the size of the stream", func() {
			buffer := bytes.NewBuffer([]byte{})
			gzipWriter := gzip.NewWriter(buffer)
			gzipWriter.Close()
			fakeSource.BlobReturns(buffer.Bytes(), 1024, nil)

			_, size, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(1024)))
		})

		Context("when the source fails to stream the blob", func() {
			It("returns an error", func() {
				fakeSource.BlobReturns(nil, 0, errors.New("failed to stream blob"))

				_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).To(MatchError(ContainSubstring("failed to stream blob")))
			})
		})

		Context("when the blob is in the cache", func() {
			var (
				blobWriter io.WriteCloser
			)

			BeforeEach(func() {
				var (
					blobReader io.ReadCloser
					err        error
				)

				blobReader, blobWriter, err = os.Pipe()
				Expect(err).NotTo(HaveOccurred())

				fakeCacheDriver.StreamBlobReturns(blobReader, 1200, nil)
			})

			JustBeforeEach(func() {
				_, err := blobWriter.Write(gzipedBlobContent)
				Expect(err).NotTo(HaveOccurred())

				Expect(blobWriter.Close()).To(Succeed())
			})

			It("calls the cache driver", func() {
				_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeCacheDriver.StreamBlobCallCount()).To(Equal(1))
				_, id, _ := fakeCacheDriver.StreamBlobArgsForCall(0)
				Expect(id).To(Equal("sha256:layer-digest"))
			})

			It("returns the cached blob", func(done Done) {
				stream, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).NotTo(HaveOccurred())

				contents, err := ioutil.ReadAll(stream)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("hello-world"))

				close(done)
			}, 2.0)

			It("returns the size of the cached blob", func() {
				_, size, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).NotTo(HaveOccurred())
				Expect(size).To(Equal(int64(1200)))
			})
		})

		Context("when the cache fails", func() {
			It("returns the error", func() {
				fakeCacheDriver.StreamBlobReturns(nil, 0, errors.New("failed to return"))

				_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).To(MatchError(ContainSubstring("failed to return")))
			})
		})
	})
})

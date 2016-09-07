package remote_test

import (
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
	"github.com/opencontainers/image-spec/specs-go"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("RemoteFetcher", func() {
	var (
		fakeCacheDriver *fetcherfakes.FakeCacheDriver
		fakeSource      *remotefakes.FakeSource
		fetcher         *remote.RemoteFetcher
		logger          *lagertest.TestLogger
		imageURL        *url.URL
	)

	BeforeEach(func() {
		fakeSource = new(remotefakes.FakeSource)
		fakeCacheDriver = new(fetcherfakes.FakeCacheDriver)

		// by default, the cache driver does not do any caching
		fakeCacheDriver.BlobStub = func(logger lager.Logger, id string,
			streamBlob fetcherpkg.StreamBlob,
		) (io.ReadCloser, error) {
			return streamBlob(logger)
		}

		fetcher = remote.NewRemoteFetcher(fakeSource, fakeCacheDriver)

		logger = lagertest.NewTestLogger("test-remote-fetcher")
		var err error
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
				fakeSource.ManifestReturns(specsv1.Manifest{}, errors.New("fetching the manifest"))
			})

			It("returns an error", func() {
				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("fetching the manifest")))
			})
		})

		It("returns the correct list of layer digests", func() {
			fakeSource.ManifestReturns(specsv1.Manifest{
				Layers: []specs.Descriptor{
					specs.Descriptor{
						MediaType: specsv1.MediaTypeImageSerialization,
						Size:      120,
						Digest:    "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
					},
					specs.Descriptor{
						MediaType: specsv1.MediaTypeImageSerialization,
						Size:      210,
						Digest:    "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
					},
				},
			}, nil)
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
			fakeSource.ManifestReturns(specsv1.Manifest{
				Config: specs.Descriptor{
					Digest: "sha256:image-digest",
				},
			}, nil)

			_, err := fetcher.ImageInfo(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeSource.ConfigCallCount()).To(Equal(1))
			_, usedImageURL, usedDigest := fakeSource.ConfigArgsForCall(0)
			Expect(usedImageURL).To(Equal(imageURL))
			Expect(usedDigest).To(Equal("sha256:image-digest"))
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

				fakeCacheDriver.BlobReturns(configReader, nil)

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
				fakeSource.ManifestReturns(specsv1.Manifest{
					Config: specs.Descriptor{
						Digest: "sha256:cached-config",
					},
				}, nil)

				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeCacheDriver.BlobCallCount()).To(Equal(1))
				_, id, _ := fakeCacheDriver.BlobArgsForCall(0)
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
				fakeCacheDriver.BlobReturns(nil, errors.New("failed to return"))

				_, err := fetcher.ImageInfo(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("failed to return")))
			})
		})
	})

	Describe("StreamBlob", func() {
		It("uses the source", func() {
			_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeSource.StreamBlobCallCount()).To(Equal(1))
			_, usedImageURL, usedDigest := fakeSource.StreamBlobArgsForCall(0)
			Expect(usedImageURL).To(Equal(imageURL))
			Expect(usedDigest).To(Equal("sha256:layer-digest"))
		})

		It("returns the stream from the source", func(done Done) {
			rEnd, wEnd, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			fakeSource.StreamBlobReturns(rEnd, 0, nil)

			stream, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
			Expect(err).NotTo(HaveOccurred())

			_, err = wEnd.Write([]byte("hello-world"))
			Expect(err).NotTo(HaveOccurred())
			Expect(wEnd.Close()).To(Succeed())

			contents, err := ioutil.ReadAll(stream)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal("hello-world"))

			close(done)
		}, 2.0)

		Context("when the source fails to stream the blob", func() {
			It("returns an error", func() {
				fakeSource.StreamBlobReturns(nil, 0, errors.New("failed to stream blob"))

				_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).To(MatchError(ContainSubstring("failed to stream blob")))
			})
		})

		Context("when the blob is in the cache", func() {
			var (
				blobWriter   io.WriteCloser
				blobContents []byte
			)

			BeforeEach(func() {
				var (
					blobReader io.ReadCloser
					err        error
				)

				blobReader, blobWriter, err = os.Pipe()
				Expect(err).NotTo(HaveOccurred())

				fakeCacheDriver.BlobReturns(blobReader, nil)

				blobContents = []byte("hello-world")
			})

			JustBeforeEach(func() {
				_, err := blobWriter.Write(blobContents)
				Expect(err).NotTo(HaveOccurred())

				Expect(blobWriter.Close()).To(Succeed())
			})

			It("calls the cache driver", func() {
				_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeCacheDriver.BlobCallCount()).To(Equal(1))
				_, id, _ := fakeCacheDriver.BlobArgsForCall(0)
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
		})

		Context("when the cache fails", func() {
			It("returns the error", func() {
				fakeCacheDriver.BlobReturns(nil, errors.New("failed to return"))

				_, _, err := fetcher.StreamBlob(logger, imageURL, "sha256:layer-digest")
				Expect(err).To(MatchError(ContainSubstring("failed to return")))
			})
		})
	})
})

package fetcher_test

import (
	"errors"
	"net/url"

	clonerpkg "code.cloudfoundry.org/grootfs/cloner"
	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/grootfs/fetcher/fetcherfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/containers/image/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/image-spec/specs-go"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Fetcher", func() {
	var (
		fakeImage         *fetcherfakes.FakeImage
		usedRef           types.ImageReference
		fakeImageProvider fetcherpkg.ImageProvider
		fetcher           *fetcherpkg.Fetcher
		logger            *lagertest.TestLogger
		cacheDriver       *fetcherfakes.FakeCacheDriver
	)

	BeforeEach(func() {
		cacheDriver = new(fetcherfakes.FakeCacheDriver)
		fakeImage = new(fetcherfakes.FakeImage)
		fakeImage.ManifestReturns(specsv1.Manifest{
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
		fakeImage.ConfigReturns(specsv1.Image{
			RootFS: specsv1.RootFS{
				DiffIDs: []string{
					"sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
					"sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
				},
			},
		}, nil)

		fakeImageProvider = func(ref types.ImageReference) fetcherpkg.Image {
			usedRef = ref
			return fakeImage
		}

		fetcher = fetcherpkg.NewFetcher(cacheDriver, fakeImageProvider)

		logger = lagertest.NewTestLogger("test-fetcher")
	})

	Describe("LayersDigest", func() {
		It("creates an image with the correct ref", func() {
			imageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.1")
			Expect(err).NotTo(HaveOccurred())

			_, err = fetcher.LayersDigest(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())

			Expect(usedRef.DockerReference().String()).To(Equal("cfgarden/empty:v0.1.1"))
		})

		It("returns the correct list of layer digests", func() {
			imageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.1")
			Expect(err).NotTo(HaveOccurred())

			digests, err := fetcher.LayersDigest(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(digests).To(Equal([]clonerpkg.LayerDigest{
				clonerpkg.LayerDigest{
					BlobID:        "sha256:47e3dd80d678c83c50cb133f4cf20e94d088f890679716c8b763418f55827a58",
					DiffID:        "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
					ChainID:       "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
					ParentChainID: "",
				},
				clonerpkg.LayerDigest{
					BlobID:        "sha256:7f2760e7451ce455121932b178501d60e651f000c3ab3bc12ae5d1f57614cc76",
					DiffID:        "sha256:d7c6a5f0d9a15779521094fa5eaf026b719984fb4bfe8e0012bd1da1b62615b0",
					ChainID:       "sha256:9242945d3c9c7cf5f127f9352fea38b1d3efe62ee76e25f70a3e6db63a14c233",
					ParentChainID: "sha256:afe200c63655576eaa5cabe036a2c09920d6aee67653ae75a9d35e0ec27205a5",
				},
			}))
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.LayersDigest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when a private registry is used", func() {
			It("creates an image with the correct ref", func() {
				imageURL, err := url.Parse("docker://my-private-registry.org/cfgarden/empty:v0.1.1")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.LayersDigest(logger, imageURL)
				Expect(err).NotTo(HaveOccurred())

				Expect(usedRef.DockerReference().String()).To(Equal("my-private-registry.org/cfgarden/empty:v0.1.1"))
			})
		})

		Context("when the image does not exist", func() {
			BeforeEach(func() {
				fakeImage.ManifestReturns(specsv1.Manifest{}, errors.New("image does not exist!"))
			})

			It("returns an error", func() {
				imageURL, err := url.Parse("docker:///non-existing/image")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.LayersDigest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("image does not exist!")))
			})
		})

		Context("when fetching the config fails", func() {
			BeforeEach(func() {
				fakeImage.ConfigReturns(specsv1.Image{}, errors.New("parsing config failed"))
			})

			It("returns an error", func() {
				imageURL, err := url.Parse("docker:///image/with-invalid-config")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.LayersDigest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing config failed")))
			})
		})
	})

	Describe("Streamer", func() {
		It("returns a streamer", func() {
			imageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.0")
			Expect(err).NotTo(HaveOccurred())

			streamer, err := fetcher.Streamer(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(streamer).NotTo(BeNil())
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.Streamer(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})
	})
})

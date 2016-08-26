package local_test

import (
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/grootfs/fetcher/local"
	"code.cloudfoundry.org/grootfs/image_puller"
	"code.cloudfoundry.org/grootfs/image_puller/image_pullerfakes"
	"code.cloudfoundry.org/lager"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	specsv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Local Fetcher", func() {
	var (
		fakeStreamer *image_pullerfakes.FakeStreamer

		fetcher   *local.LocalFetcher
		imagePath string
		imageSrc  *url.URL
		logger    lager.Logger
	)

	BeforeEach(func() {
		var err error
		logger = lager.NewLogger("local-fetcher-test")

		imagePath, err = ioutil.TempDir("", "image")
		Expect(err).NotTo(HaveOccurred())

		imageSrc, err = url.Parse(imagePath)
		Expect(err).NotTo(HaveOccurred())

		fakeStreamer = new(image_pullerfakes.FakeStreamer)

		fetcher = local.NewLocalFetcher(fakeStreamer)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(imagePath)).To(Succeed())
	})

	Describe("Streamer", func() {
		It("returns a streamer", func() {
			streamer, err := fetcher.Streamer(logger, imageSrc)
			Expect(err).NotTo(HaveOccurred())
			Expect(streamer).NotTo(BeNil())
		})

		Context("when the image path does not exist", func() {
			BeforeEach(func() {
				var err error
				imageSrc, err = url.Parse("invalid-path")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := fetcher.Streamer(logger, imageSrc)
				Expect(err).To(MatchError(ContainSubstring("image source does not exist")))
			})
		})

		Context("when fails to access image path", func() {
			var noPermImageURL *url.URL

			JustBeforeEach(func() {
				var err error
				noPermImagePath := filepath.Join(imagePath, "no-perm-dir")
				Expect(os.Mkdir(noPermImagePath, 0000)).To(Succeed())
				Expect(os.Chmod(imagePath, 0000)).To(Succeed())
				noPermImageURL, err = url.Parse(noPermImagePath)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(os.Chmod(imagePath, 0700)).To(Succeed())
			})

			It("returns an error", func() {
				_, err := fetcher.Streamer(logger, noPermImageURL)
				Expect(err).To(MatchError(ContainSubstring("failed to access image path")))
			})
		})
	})

	Describe("LayersDigest", func() {
		var imageInfo image_puller.ImageInfo

		BeforeEach(func() {
			var err error
			imageInfo, err = fetcher.ImageInfo(logger, imageSrc)
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
				newImageInfo, err := fetcher.ImageInfo(logger, imageSrc)
				Expect(err).NotTo(HaveOccurred())
				Expect(imageInfo.LayersDigest[0].ChainID).NotTo(Equal(newImageInfo.LayersDigest[0].ChainID))
			})
		})

		Context("when the image doesn't exist", func() {
			BeforeEach(func() {
				var err error
				imageSrc, err = url.Parse("/not-here")
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns an error", func() {
				_, err := fetcher.ImageInfo(logger, imageSrc)
				Expect(err).To(MatchError(ContainSubstring("fetching image timestamp")))
			})
		})
	})
})

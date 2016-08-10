package fetcher_test

import (
	"net/url"

	fetcherpkg "code.cloudfoundry.org/grootfs/fetcher"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetcher", func() {
	var (
		fetcher *fetcherpkg.Fetcher
		logger  *lagertest.TestLogger
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test-fetcher")
		fetcher = fetcherpkg.NewFetcher()
	})

	Describe("LayersDigest", func() {
		It("returns the correct list of layer digests", func() {
			expectedDigests := []string{
				"sha256:6c1f4533b125f8f825188c4f4ff633a338cfce0db2813124d3d518028baf7d7a",
			}

			imageURL, err := url.Parse("docker:///cfgarden/empty:v0.1.0")
			Expect(err).NotTo(HaveOccurred())

			digests, err := fetcher.LayersDigest(logger, imageURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(digests).To(Equal(expectedDigests))
		})

		Context("when the image url is invalid", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:cfgarden/empty:v0.1.0")
				Expect(err).NotTo(HaveOccurred())

				_, err = fetcher.LayersDigest(logger, imageURL)
				Expect(err).To(MatchError(ContainSubstring("parsing url failed")))
			})
		})

		Context("when the image doesn't exist", func() {
			It("returns an error", func() {
				imageURL, err := url.Parse("docker:///cfgarden/empty:ImNotGroot")
				Expect(err).NotTo(HaveOccurred())

				digests, err := fetcher.LayersDigest(logger, imageURL)
				Expect(digests).To(BeNil())
				Expect(err).To(MatchError(ContainSubstring("inspecting image")))
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
